import asyncio
import json
import logging
import traceback

import httpx
import pytest
from pydantic import ValidationError

from app.clients.blog import BlogClient
from app.clients.errors import (
    ArticleNotFoundError,
    AuthenticationError,
    AuthorizationError,
    BlogBackendError,
    BlogBackendUnavailableError,
    BlogRequestError,
    VersionConflictError,
)
from app.clients.models import Article, CreateDraftInput, UpdateDraftInput
from app.core.config import Settings


@pytest.fixture
def anyio_backend():
    return "asyncio"


@pytest.fixture
def settings():
    return Settings(
        _env_file=None, blog_backend_url="http://blog.test:8080", blog_backend_timeout_seconds=3.5
    )


@pytest.fixture
def article_data():
    return {
        "id": 18, "status": "published", "version": 6, "published_version": 5,
        "title": "Working title", "content": "<h2>Heading</h2><p>Working V6</p>",
        "summary": "Summary", "cover_image": "/uploads/cover.png",
        "tags": [{"id": 2, "name": "Go"}],
        "view_count": 12, "like_count": 3, "comment_count": 1,
        "created_at": "2026-09-15T10:00:00Z", "updated_at": "2026-09-15T11:00:00Z",
        "user": {
            "id": 7, "username": "owner", "email": "owner@example.com", "avatar": "",
            "bio": "", "role": "user", "is_active": True, "created_at": "2026-09-01T10:00:00Z",
        },
    }


def success(article, status=200):
    return httpx.Response(status, json={"code": status, "message": "success", "data": article})


@pytest.mark.anyio
async def test_get_uses_agent_contract_and_request_token(settings, article_data):
    def handler(request):
        assert request.method == "GET"
        assert str(request.url) == "http://blog.test:8080/api/v1/agent/articles/18"
        assert request.headers["Authorization"] == "Bearer user-token-A"
        assert request.content == b""
        assert request.extensions["timeout"] == dict.fromkeys(("connect", "read", "write", "pool"), 3.5)
        return success(article_data)

    async with BlogClient(settings, transport=httpx.MockTransport(handler)) as client:
        article = await client.get_article(18, access_token="user-token-A")
    assert isinstance(article, Article)
    assert article.id == 18
    assert (article.version, article.published_version) == (6, 5)
    assert article.content == article_data["content"]
    assert article.tags[0].name == "Go"
    assert article.user.id == 7
    assert article.created_at.year == 2026


@pytest.mark.anyio
async def test_all_business_routes_and_partial_update_payloads(settings, article_data):
    requests = []

    def handler(request):
        requests.append(request)
        if request.method == "GET":
            return httpx.Response(200, json={
                "code": 200, "message": "success", "data": [article_data],
                "meta": {"page": 2, "limit": 5, "total": 6, "pages": 2},
            })
        return success(article_data, 201 if request.url.path.endswith("/articles") else 200)

    async with BlogClient(settings, transport=httpx.MockTransport(handler)) as client:
        page = await client.list_my_articles(access_token="token", page=2, limit=5, status="published")
        assert page.meta.total == 6 and page.meta.pages == 2
        assert page.data[0].version == 6
        await client.create_draft(CreateDraftInput(title="Draft", content="<p>Draft</p>"), access_token="token")
        await client.update_draft(18, UpdateDraftInput(expected_version=6, content="New"), access_token="token")
        await client.update_draft(
            18, UpdateDraftInput(expected_version=6, summary="", cover_image="", tag_ids=[]), access_token="token"
        )
        await client.publish_article(18, expected_version=6, access_token="token")
        await client.archive_article(18, access_token="token")

    root = "/api/v1/agent/articles"
    assert [(r.method, r.url.path) for r in requests] == [
        ("GET", root), ("POST", root), ("PUT", root + "/18/draft"),
        ("PUT", root + "/18/draft"), ("POST", root + "/18/publish"), ("POST", root + "/18/archive"),
    ]
    assert dict(requests[0].url.params) == {"page": "2", "limit": "5", "status": "published"}
    assert json.loads(requests[1].content) == {
        "title": "Draft", "content": "<p>Draft</p>", "summary": "", "cover_image": "", "tag_ids": [],
    }
    assert json.loads(requests[2].content) == {"expected_version": 6, "content": "New"}
    assert json.loads(requests[3].content) == {"expected_version": 6, "summary": "", "cover_image": "", "tag_ids": []}
    assert json.loads(requests[4].content) == {"expected_version": 6}
    assert requests[5].content == b""
    for request in requests:
        assert request.headers["Authorization"] == "Bearer token"
        assert "access_token" not in request.url.params
        assert "user_id" not in request.url.params


@pytest.mark.anyio
async def test_tokens_do_not_mix_during_concurrent_or_later_calls(settings, article_data):
    arrived = 0
    ready = asyncio.Event()
    seen = []

    async def handler(request):
        nonlocal arrived
        arrived += 1
        if arrived == 2:
            ready.set()
        await asyncio.wait_for(ready.wait(), timeout=2)
        token = request.headers["Authorization"]
        seen.append(token)
        return success({**article_data, "title": token.removeprefix("Bearer ")})

    async with BlogClient(settings, transport=httpx.MockTransport(handler)) as client:
        a, b = await asyncio.gather(
            client.get_article(18, access_token="token-A"),
            client.get_article(18, access_token="token-B"),
        )
        c = await client.get_article(18, access_token="token-C")
    assert (a.title, b.title, c.title) == ("token-A", "token-B", "token-C")
    assert sorted(seen) == ["Bearer token-A", "Bearer token-B", "Bearer token-C"]


@pytest.mark.parametrize("field", ["user_id", "status", "version", "published_version", "view_count", "like_count", "comment_count"])
def test_draft_inputs_forbid_non_business_fields(field):
    with pytest.raises(ValidationError):
        CreateDraftInput.model_validate({"title": "Draft", "content": "Draft", field: 99})
    with pytest.raises(ValidationError):
        UpdateDraftInput.model_validate({"expected_version": 6, field: 99})


@pytest.mark.parametrize("value", [None, 0, -1, 1.5, "6", True])
def test_update_requires_a_positive_integer_version(value):
    with pytest.raises(ValidationError):
        UpdateDraftInput(expected_version=value)


def test_update_requires_version_even_when_no_content_is_changed():
    with pytest.raises(ValidationError):
        UpdateDraftInput()


@pytest.mark.anyio
@pytest.mark.parametrize("operation", ["update", "publish"])
@pytest.mark.parametrize("status,error_type", [
    (400, BlogRequestError), (401, AuthenticationError), (403, AuthorizationError),
    (404, ArticleNotFoundError), (409, VersionConflictError),
    (500, BlogBackendError), (503, BlogBackendError),
])
async def test_http_errors_keep_safe_context_and_never_retry(settings, status, error_type, operation):
    calls = 0
    message = "文章已被其他操作更新，请刷新后重新编辑"

    def handler(request):
        nonlocal calls
        calls += 1
        return httpx.Response(status, json={"code": status, "message": message})

    async with BlogClient(settings, transport=httpx.MockTransport(handler)) as client:
        with pytest.raises(error_type) as caught:
            if operation == "update":
                await client.update_draft(18, UpdateDraftInput(expected_version=5, content="New"), access_token="private-jwt")
            else:
                await client.publish_article(18, expected_version=5, access_token="private-jwt")
    error = caught.value
    assert type(error) is error_type
    assert error.status_code == status
    assert error.method == ("PUT" if operation == "update" else "POST")
    assert error.path == "/api/v1/agent/articles/18/" + ("draft" if operation == "update" else "publish")
    if status < 500:
        assert str(error) == message
    assert calls == 1


@pytest.mark.anyio
@pytest.mark.parametrize("error_type", [httpx.ReadTimeout, httpx.ConnectError])
@pytest.mark.parametrize("operation", ["create", "update", "publish", "archive"])
async def test_transport_errors_hide_token_and_do_not_retry(settings, error_type, operation, caplog):
    caplog.set_level(logging.INFO)
    calls = 0
    token = "secret-user-jwt"

    def handler(request):
        nonlocal calls
        calls += 1
        raise error_type(f"Transport error with {token}", request=request)

    async with BlogClient(settings, transport=httpx.MockTransport(handler)) as client:
        with pytest.raises(BlogBackendUnavailableError) as caught:
            if operation == "create":
                await client.create_draft(CreateDraftInput(title="Draft", content="Draft"), access_token=token)
            elif operation == "update":
                await client.update_draft(18, UpdateDraftInput(expected_version=5, content="New"), access_token=token)
            elif operation == "publish":
                await client.publish_article(18, expected_version=5, access_token=token)
            else:
                await client.archive_article(18, access_token=token)
    assert calls == 1
    assert caught.value.status_code is None
    assert token not in "".join(traceback.format_exception(caught.value))
    assert token not in caplog.text


@pytest.mark.anyio
@pytest.mark.parametrize("status", [401, 409, 500])
async def test_error_messages_and_logs_do_not_expose_forwarded_jwt(settings, status, caplog):
    token = "private-user.jwt.signature"
    caplog.set_level(logging.INFO)

    def handler(request):
        return httpx.Response(status, json={"code": status, "message": f"Rejected Bearer {token}"})

    async with BlogClient(settings, transport=httpx.MockTransport(handler)) as client:
        with pytest.raises((AuthenticationError, VersionConflictError, BlogBackendError)) as caught:
            await client.get_article(18, access_token=token)
    assert token not in str(caught.value)
    assert token not in repr(vars(caught.value))
    assert token not in caplog.text


@pytest.mark.anyio
async def test_redirect_is_not_followed(settings):
    calls = 0

    def handler(request):
        nonlocal calls
        calls += 1
        return httpx.Response(307, headers={"Location": "http://elsewhere.test/collect"})

    async with BlogClient(settings, transport=httpx.MockTransport(handler)) as client:
        with pytest.raises(BlogBackendError) as caught:
            await client.publish_article(18, expected_version=6, access_token="token")
    assert caught.value.status_code == 307
    assert calls == 1


@pytest.mark.anyio
@pytest.mark.parametrize("payload", [b"not-json", b"[]", b'{"code":200}', b'{"code":200,"data":{}}', b'{"code":409,"data":{}}'])
async def test_invalid_success_response_has_a_controlled_error(settings, payload):
    async with BlogClient(settings, transport=httpx.MockTransport(lambda request: httpx.Response(200, content=payload))) as client:
        with pytest.raises(BlogBackendError, match="invalid response"):
            await client.get_article(18, access_token="token")


@pytest.mark.anyio
async def test_missing_optional_tags_and_empty_list(settings, article_data):
    article_data.pop("tags")
    article_data.pop("user")

    def handler(request):
        if request.url.path.endswith("/18"):
            return success(article_data)
        return httpx.Response(200, json={
            "code": 200, "message": "success", "data": [],
            "meta": {"page": 1, "limit": 10, "total": 0, "pages": 0},
        })

    async with BlogClient(settings, transport=httpx.MockTransport(handler)) as client:
        article = await client.get_article(18, access_token="token")
        page = await client.list_my_articles(access_token="token")
    assert article.tags == [] and article.user is None
    assert page.data == [] and page.meta.total == 0


@pytest.mark.anyio
async def test_invalid_arguments_never_send_http(settings):
    def handler(request):
        pytest.fail("Invalid argument reached HTTP transport")

    async with BlogClient(settings, transport=httpx.MockTransport(handler)) as client:
        for value in (0, -1, "18/../other", True):
            with pytest.raises(ValueError):
                await client.get_article(value, access_token="token")
        for token in ("", "Bearer token", "token\r\nInjected: header"):
            with pytest.raises(ValueError):
                await client.get_article(18, access_token=token)
        for value in (0, -1, "6", True):
            with pytest.raises(ValueError):
                await client.publish_article(18, expected_version=value, access_token="token")
        with pytest.raises(ValueError):
            await client.list_my_articles(access_token="token", status="invalid")


@pytest.mark.anyio
async def test_client_closes_transport_on_exit_and_explicit_close(settings):
    class TrackedTransport(httpx.MockTransport):
        closed = False

        async def aclose(self):
            self.closed = True

    transport = TrackedTransport(lambda request: httpx.Response(500))
    with pytest.raises(RuntimeError, match="caller failed"):
        async with BlogClient(settings, transport=transport):
            raise RuntimeError("caller failed")
    assert transport.closed
    transport = TrackedTransport(lambda request: httpx.Response(500))
    client = BlogClient(settings, transport=transport)
    await client.aclose()
    await client.aclose()
    assert transport.closed

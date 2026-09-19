import httpx
import pytest

from app.clients.blog import BlogClient
from app.clients.errors import (
    ArticleNotFoundError, AuthenticationError, AuthorizationError, BlogBackendError,
    BlogBackendUnavailableError, BlogRequestError,
)
from app.core.config import Settings


@pytest.fixture
def anyio_backend():
    return "asyncio"


def version_data(article_id=23, version_no=7):
    return {
        "article_id": article_id, "version_no": version_no, "title": "Redis",
        "content": "<h1>Redis</h1><p><strong>旧版</strong>正文</p><p>保留段落</p>",
        "summary": "摘要", "cover_image": "/cover.png",
    }


@pytest.mark.anyio
async def test_exact_historical_version_request_and_response():
    def handler(request):
        assert request.method == "GET"
        assert request.url.path == "/api/v1/agent/articles/23/versions/7"
        assert not request.url.query and not request.content
        assert request.headers["Authorization"] == "Bearer fake-token"
        return httpx.Response(200, json={"code": 200, "data": version_data()})

    async with BlogClient(Settings(_env_file=None), transport=httpx.MockTransport(handler)) as client:
        snapshot = await client.get_article_version(23, 7, access_token="fake-token")
    assert snapshot.model_dump() == version_data()


@pytest.mark.anyio
@pytest.mark.parametrize("status,error_type", [
    (400, BlogRequestError), (401, AuthenticationError), (403, AuthorizationError),
    (404, ArticleNotFoundError), (500, BlogBackendError),
])
async def test_version_read_reuses_safe_error_mapping(status, error_type):
    calls = []

    def handler(request):
        calls.append(request)
        return httpx.Response(status, json={"code": status, "message": "fake-token"})

    async with BlogClient(Settings(_env_file=None), transport=httpx.MockTransport(handler)) as client:
        with pytest.raises(error_type) as error:
            await client.get_article_version(23, 7, access_token="fake-token")
    assert len(calls) == 1 and "fake-token" not in str(error.value)


@pytest.mark.anyio
@pytest.mark.parametrize("failure", ["article", "version", "content", "transport"])
async def test_version_read_rejects_mismatched_or_invalid_response(failure):
    data = version_data()
    if failure == "article": data["article_id"] = 24
    if failure == "version": data["version_no"] = 8
    if failure == "content": data.pop("content")

    def handler(request):
        if failure == "transport": raise httpx.ConnectError("private-details", request=request)
        return httpx.Response(200, json={"code": 200, "data": data})

    async with BlogClient(Settings(_env_file=None), transport=httpx.MockTransport(handler)) as client:
        with pytest.raises(BlogBackendUnavailableError if failure == "transport" else BlogBackendError):
            await client.get_article_version(23, 7, access_token="fake-token")


@pytest.mark.anyio
@pytest.mark.parametrize("article,version", [(0, 1), (1, 0), (True, 1), (1, -1), (1, "7")])
async def test_version_read_invalid_identity_never_reaches_go(article, version):
    def handler(request):
        pytest.fail("Invalid identity reached Go")

    async with BlogClient(Settings(_env_file=None), transport=httpx.MockTransport(handler)) as client:
        with pytest.raises(ValueError):
            await client.get_article_version(article, version, access_token="fake-token")

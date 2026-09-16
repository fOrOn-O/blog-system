import asyncio
import json

import httpx
import pytest
from langchain_core.messages import AIMessage, ToolMessage

from app.agent.runner import AgentRunner
from app.agent.tools import AGENT_TOOLS
from app.clients.blog import BlogClient
from app.clients.errors import (
    ArticleNotFoundError, AuthenticationError, AuthorizationError, BlogBackendError,
    BlogBackendUnavailableError, BlogRequestError, VersionConflictError,
)
from app.clients.models import ArticleVersionDiff
from app.core.config import Settings


@pytest.fixture
def anyio_backend():
    return "asyncio"


@pytest.fixture(autouse=True)
def disable_tracing(monkeypatch):
    monkeypatch.setenv("LANGSMITH_TRACING", "false")
    monkeypatch.setenv("LANGCHAIN_TRACING_V2", "false")


@pytest.fixture
def settings():
    return Settings(_env_file=None, groq_api_key="", blog_backend_url="http://blog.test:8080")


@pytest.fixture
def diff_data():
    return {
        "article_id": 18, "from_version": 2, "to_version": 6,
        "field_changes": {
            "title": {"changed": True, "before": "Old", "after": "New"},
            "summary": {"changed": False, "before": "", "after": ""},
            "cover_image": {"changed": False, "before": "", "after": ""},
        },
        "content": {"changed": True, "changes": [
            {"operation": "modify", "before_index": 0, "after_index": 0,
             "before": {"type": "paragraph", "text": "Redis很快"},
             "after": {"type": "paragraph", "text": "Redis性能很高"}},
            {"operation": "delete", "before_index": 1, "after_index": None,
             "before": {"type": "heading", "text": "Old heading"}, "after": None},
            {"operation": "insert", "before_index": None, "after_index": 1,
             "before": None, "after": {"type": "code_block", "text": " x\n  y"}},
        ]},
    }


@pytest.mark.anyio
async def test_diff_client_request_and_response(settings, diff_data):
    def handler(request):
        assert request.method == "GET"
        assert request.url.path == "/api/v1/agent/articles/18/diff"
        assert dict(request.url.params) == {"from_version": "2", "to_version": "6"}
        assert request.headers["Authorization"] == "Bearer test-user-token"
        assert request.content == b""
        return httpx.Response(200, json={"code": 200, "data": diff_data})

    async with BlogClient(settings, transport=httpx.MockTransport(handler)) as client:
        result = await client.get_version_diff(18, 2, 6, access_token="test-user-token")
    assert isinstance(result, ArticleVersionDiff)
    assert result.model_dump(mode="json") == diff_data


@pytest.mark.anyio
@pytest.mark.parametrize("status,error", [
    (400, BlogRequestError), (401, AuthenticationError), (403, AuthorizationError),
    (404, ArticleNotFoundError), (409, VersionConflictError), (500, BlogBackendError),
])
async def test_diff_backend_error_mapping(settings, status, error):
    requests = []

    def handler(request):
        requests.append(request)
        return httpx.Response(status, json={"code": status, "message": "rejected private-token"})

    async with BlogClient(settings, transport=httpx.MockTransport(handler)) as client:
        with pytest.raises(error) as caught:
            await client.get_version_diff(18, 2, 6, access_token="private-token")
    assert caught.value.status_code == status
    assert caught.value.path == "/api/v1/agent/articles/18/diff"
    assert caught.value.method == "GET"
    assert "private-token" not in str(caught.value)
    assert len(requests) == 1


@pytest.mark.anyio
async def test_diff_transport_and_invalid_response_mapping(settings):
    def fail(request):
        raise httpx.ConnectError("private-token", request=request)

    for handler, error in [
        (fail, BlogBackendUnavailableError),
        (lambda request: httpx.Response(200, json={"code": 200, "data": {}}), BlogBackendError),
    ]:
        async with BlogClient(settings, transport=httpx.MockTransport(handler)) as client:
            with pytest.raises(error) as caught:
                await client.get_version_diff(18, 2, 6, access_token="private-token")
        assert "private-token" not in str(caught.value)


@pytest.mark.anyio
@pytest.mark.parametrize("article_id,from_version,to_version", [
    (0, 1, 2), (18, 0, 2), (18, 2, 0), (18, 2, 2), (18, 3, 2),
    (18, True, 2), (18, "1", 2), (18, 1, 2.5),
])
async def test_diff_rejects_invalid_input_before_http(settings, article_id, from_version, to_version):
    def forbidden(request):
        pytest.fail("Invalid diff input must not reach Go")

    async with BlogClient(settings, transport=httpx.MockTransport(forbidden)) as client:
        with pytest.raises(ValueError):
            await client.get_version_diff(article_id, from_version, to_version, access_token="token")


class DiffModel:
    """仅使用假模型；Graph、Runtime、工具和 BlogClient 均使用真实实现。"""

    def __init__(self, args=None):
        self.args = args if args is not None else {"article_id": 18, "from_version": 2, "to_version": 6}
        self.inputs = []

    def bind_tools(self, tools):
        self.tools = tools
        return self

    async def ainvoke(self, messages):
        self.inputs.append(messages)
        if isinstance(messages[-1], ToolMessage):
            return AIMessage(content=messages[-1].content)
        return AIMessage(content="", tool_calls=[{
            "name": "get_version_diff", "args": self.args, "id": "diff-call",
        }])


def test_diff_tool_schema_excludes_identity():
    tool = next(t for t in AGENT_TOOLS if t.name == "get_version_diff")
    schema = tool.tool_call_schema.model_json_schema()
    expected = {"article_id", "from_version", "to_version"}
    assert set(schema["properties"]) == set(schema["required"]) == expected
    for name in ("jwt", "access_token", "user_id", "runtime", "blog_client"):
        assert name not in json.dumps(schema).lower()


@pytest.mark.anyio
async def test_diff_runtime_concurrent_jwt_isolation(settings, diff_data):
    model = DiffModel()
    runner = AgentRunner(model, settings=settings)
    seen = []
    ready = asyncio.Event()

    async def handler(request):
        seen.append(request.headers["Authorization"])
        assert request.url.path == "/api/v1/agent/articles/18/diff"
        assert dict(request.url.params) == {"from_version": "2", "to_version": "6"}
        assert request.method == "GET"
        if len(seen) == 2:
            ready.set()
        await asyncio.wait_for(ready.wait(), 2)
        data = json.loads(json.dumps(diff_data))
        data["field_changes"]["title"]["after"] = "User A" if seen and request.headers["Authorization"] == "Bearer token-A" else "User B"
        return httpx.Response(200, json={"code": 200, "data": data})

    async with BlogClient(settings, transport=httpx.MockTransport(handler)) as client:
        a, b = await asyncio.gather(
            runner.run("Compare versions", access_token="token-A", blog_client=client),
            runner.run("Compare versions", access_token="token-B", blog_client=client),
        )
    assert json.loads(a.content)["data"]["field_changes"]["title"]["after"] == "User A"
    assert json.loads(b.content)["data"]["field_changes"]["title"]["after"] == "User B"
    assert sorted(seen) == ["Bearer token-A", "Bearer token-B"]
    messages = json.dumps([[m.model_dump(mode="json") for m in turn] for turn in model.inputs])
    assert "token-A" not in messages and "token-B" not in messages


@pytest.mark.anyio
async def test_diff_tool_permission_error_is_safe(settings):
    def handler(request):
        return httpx.Response(403, json={"code": 403, "message": "private content and private-token"})

    model = DiffModel()
    async with BlogClient(settings, transport=httpx.MockTransport(handler)) as client:
        result = await AgentRunner(model, settings=settings).run("Compare", access_token="private-token", blog_client=client)
    payload = json.loads(result.content)
    assert payload["ok"] is False and payload["error"] == "permission_denied"
    assert "private content" not in result.content and "private-token" not in result.content


@pytest.mark.anyio
async def test_diff_tool_invalid_order_never_reaches_http(settings):
    def forbidden(request):
        pytest.fail("Invalid order must not issue a request")

    model = DiffModel({"article_id": 18, "from_version": 6, "to_version": 2})
    async with BlogClient(settings, transport=httpx.MockTransport(forbidden)) as client:
        result = await AgentRunner(model, settings=settings).run("Compare", access_token="token", blog_client=client)
    assert json.loads(result.content)["error"] == "invalid_arguments"

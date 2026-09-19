import asyncio
import json
from unittest.mock import AsyncMock

import httpx
import pytest
from langchain_core.messages import AIMessage, ToolMessage
from qdrant_client import AsyncQdrantClient

from app.agent.runner import AgentRunner
from app.agent.tools import AGENT_TOOLS
from app.clients.blog import BlogClient
from app.clients.errors import (
    ArticleNotFoundError, AuthenticationError, AuthorizationError, BlogBackendError,
    BlogBackendUnavailableError, BlogRequestError, VersionConflictError,
)
from app.core.config import Settings
from app.rag.service import ArticleRagService
from app.rag.store import QdrantChunkStore
from test_agent import ScriptedModel, assert_no_token_in_model_inputs, tool_call
from test_rag import FakeEmbedding, source


@pytest.fixture
def anyio_backend():
    return "asyncio"


@pytest.fixture(autouse=True)
def no_external_tracing(monkeypatch):
    monkeypatch.setenv("LANGSMITH_TRACING", "false")
    monkeypatch.setenv("LANGCHAIN_TRACING_V2", "false")


@pytest.mark.anyio
async def test_blog_client_chunks_mapping_and_jwt_isolation():
    settings = Settings(_env_file=None)
    requests = []

    async def handler(request):
        requests.append(request)
        assert request.method == "GET" and request.url.path == "/api/v1/agent/articles/18/versions/1/chunks"
        assert not request.url.query and not request.content
        user = 7 if request.headers["Authorization"] == "Bearer token-A" else 8
        await asyncio.sleep(0)
        return httpx.Response(200, json={"code": 200, "data": source(user=user).model_dump()})

    async with BlogClient(settings, transport=httpx.MockTransport(handler)) as client:
        a, b = await asyncio.gather(*[client.get_version_chunks(18, 1, access_token=token) for token in ("token-A", "token-B")])
    assert a.user_id == 7 and b.user_id == 8
    assert a.model_dump() == source().model_dump()
    assert {r.headers["Authorization"] for r in requests} == {"Bearer token-A", "Bearer token-B"}


@pytest.mark.anyio
@pytest.mark.parametrize("status,error_type", [
    (400, BlogRequestError), (401, AuthenticationError), (403, AuthorizationError),
    (404, ArticleNotFoundError), (409, VersionConflictError), (500, BlogBackendError),
])
async def test_chunks_error_mapping(status, error_type):
    calls = []

    def handler(request):
        calls.append(request)
        return httpx.Response(status, json={"code": status, "message": "token-A"})

    async with BlogClient(Settings(_env_file=None), transport=httpx.MockTransport(handler)) as client:
        with pytest.raises(error_type) as error:
            await client.get_version_chunks(18, 1, access_token="token-A")
    assert "token-A" not in str(error.value)
    assert len(calls) == 1


@pytest.mark.anyio
@pytest.mark.parametrize("invalid", ["article", "version", "indices", "identity", "transport"])
async def test_chunks_contract_and_transport_errors(invalid):
    payload = source().model_dump()
    if invalid == "article": payload["article_id"] = 99
    if invalid == "version": payload["version_no"] = 2
    if invalid == "indices": payload["chunks"][0]["index"] = 2
    if invalid == "identity": payload.pop("user_id")

    def handler(request):
        if invalid == "transport": raise httpx.ConnectError("private details", request=request)
        return httpx.Response(200, json={"code": 200, "data": payload})

    async with BlogClient(Settings(_env_file=None), transport=httpx.MockTransport(handler)) as client:
        with pytest.raises(BlogBackendUnavailableError if invalid == "transport" else BlogBackendError):
            await client.get_version_chunks(18, 1, access_token="token-A")


@pytest.mark.anyio
@pytest.mark.filterwarnings("ignore:Payload indexes have no effect in the local Qdrant:UserWarning")
async def test_real_graph_retrieval_context_and_concurrent_runtime_jwt(monkeypatch):
    settings = Settings(_env_file=None, rag_top_k=2)
    store = QdrantChunkStore(AsyncQdrantClient(":memory:"), settings)
    service = ArticleRagService(FakeEmbedding(settings), store, top_k=settings.rag_top_k)
    requests = []

    async def handler(request):
        token = request.headers["Authorization"]
        requests.append((request.url.path, token))
        is_a = token == "Bearer token-A"
        article, user = (18, 7) if is_a else (19, 8)
        await asyncio.sleep(0)
        if request.url.path != f"/api/v1/agent/articles/{article}/versions/1/chunks":
            return httpx.Response(403, json={"code": 403, "message": "Forbidden"})
        data = source(3, article=article, user=user)
        for chunk in data.chunks:
            chunk.text = f"Owner {user} / {chunk.text}"
        return httpx.Response(200, json={"code": 200, "data": data.model_dump()})

    async with BlogClient(settings, transport=httpx.MockTransport(handler)) as blog:
        try:
            for article, token in [(18, "token-A"), (19, "token-B")]:
                await service.index_article_version(article, 1, access_token=token, blog_client=blog)
            # 工具执行只检索，不会调用索引方法。
            index = AsyncMock(side_effect=AssertionError("tool must not index"))
            monkeypatch.setattr(service, "index_article_version", index)

            class AnswerFromContext(ScriptedModel):
                async def ainvoke(self, messages):
                    if isinstance(messages[-1], ToolMessage):
                        data = json.loads(messages[-1].content)["data"]
                        assert data["context"].index("Chunk 2") < data["context"].index("Chunk 1")
                        self.responses.append(AIMessage(content=data["chunks"][0]["text"]))
                    return await super().ainvoke(messages)

            models = [AnswerFromContext([tool_call("search_article_version", {"article_id": article, "version_no": 1, "query": "Redis"})]) for article in (18, 19)]
            answers = await asyncio.gather(*[
                AgentRunner(model, settings=settings, rag_service=service).run("检索指定文章版本", access_token=token, blog_client=blog)
                for model, token in zip(models, ("token-A", "token-B"))
            ])
            assert "Owner 7" in answers[0].content and "Owner 8" not in answers[0].content
            assert "Owner 8" in answers[1].content and "Owner 7" not in answers[1].content
            for model in models:
                assert_no_token_in_model_inputs(model, "token-A")
                assert_no_token_in_model_inputs(model, "token-B")
                data = json.loads(model.inputs[1][-1].content)["data"]
                assert len(data["chunks"]) == 2 and "user_id" not in json.dumps(data)
            forbidden = ScriptedModel([
                tool_call("search_article_version", {"article_id": 19, "version_no": 1, "query": "Redis"}),
                AIMessage(content="无权访问"),
            ])
            await AgentRunner(forbidden, settings=settings, rag_service=service).run("读他人的文章", access_token="token-A", blog_client=blog)
            result = json.loads(forbidden.inputs[1][-1].content)
            assert result["error"] == "permission_denied" and "Owner 8" not in json.dumps(result)
            assert len(requests) == 5
            index.assert_not_called()
        finally:
            await store.aclose()


def test_rag_tool_schema_excludes_identity_and_indexing():
    tool = next(t for t in AGENT_TOOLS if t.name == "search_article_version")
    assert set(tool.tool_call_schema.model_json_schema()["properties"]) == {"article_id", "version_no", "query"}
    assert not {"index_article_version", "publish_article", "archive_article"} & {t.name for t in AGENT_TOOLS}

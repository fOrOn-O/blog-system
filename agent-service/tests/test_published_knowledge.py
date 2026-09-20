import copy
import json
from unittest.mock import AsyncMock, Mock

import httpx
import pytest
from fastapi import FastAPI
from fastapi.testclient import TestClient
from langchain_core.messages import AIMessage
from qdrant_client import models
from qdrant_client.http.models import QueryResponse

from app.api import knowledge
from app.clients.blog import BlogClient
from app.clients.errors import AuthenticationError, AuthorizationError, BlogBackendError
from app.clients.models import PublishedKnowledgeRecord
from app.core.config import Settings
from app.core import rag as factory
from app.knowledge.service import PublishedKnowledgeService, published_payload, published_point_id
from app.knowledge.answer import grounded_answer
from app.knowledge.models import KnowledgeHit
from app.rag.errors import RagError
from app.rag.store import QdrantChunkStore
from qdrant_client import AsyncQdrantClient
from test_rag import source, FakeEmbedding
from test_rag_cloud import cloud, cloud_settings


@pytest.fixture
def anyio_backend():
    return "asyncio"


def published(article=18, version=5, count=3):
    return PublishedKnowledgeRecord(article_id=article, published_version=version, title=f"文章 {article}", chunks=source(count).chunks)


def seed(cloud, record):
    for c in record.chunks:
        cloud.points[published_point_id(record.article_id, record.published_version, c.index)] = published_payload(record, c)


def blog_source(records):
    async def get(*, access_token, article_id=None):
        assert access_token == "private-jwt"
        return [r for r in records if article_id is None or r.article_id == article_id]
    return Mock(get_published_knowledge=AsyncMock(side_effect=get))


@pytest.fixture
def service(cloud):
    original = cloud.client.scroll.side_effect
    async def scroll(collection, **kwargs):
        if kwargs["scroll_filter"] is None:
            kwargs["scroll_filter"] = models.Filter(must=[])
        return await original(collection, **kwargs)
    cloud.client.scroll.side_effect = scroll
    return PublishedKnowledgeService(cloud.backend.store, top_k=3)


@pytest.mark.anyio
async def test_republish_replaces_only_current_version_after_verify(cloud, service):
    old, current, other = published(), published(version=7, count=2), published(article=19)
    seed(cloud, old)
    seed(cloud, other)
    blog = blog_source([current, other])
    await service.sync_article(18, access_token="private-jwt", blog_client=blog)
    assert all(p["version_no"] == 7 for p in cloud.points.values() if p["article_id"] == 18)
    assert len(cloud.points) == 5
    assert cloud.events.index("verify") < cloud.events.index("delete")
    for p in cloud.client.upsert.call_args.kwargs["points"]:
        assert isinstance(p.vector, models.Document)
        assert p.vector.text == p.payload["text"] and p.vector.model == "intfloat/multilingual-e5-small"
        assert "user_id" not in p.payload
    previous = copy.deepcopy(cloud.points)
    await service.sync_article(18, access_token="private-jwt", blog_client=blog)
    assert cloud.points == previous
    blog.get_published_knowledge.assert_awaited_with(article_id=18, access_token="private-jwt")


@pytest.mark.anyio
@pytest.mark.parametrize("stage", ["upsert", "retrieve"])
async def test_sync_failure_keeps_old_active_version_and_retry_recovers(cloud, service, stage):
    seed(cloud, published())
    original = getattr(cloud.client, stage).side_effect
    getattr(cloud.client, stage).side_effect = RuntimeError("secret-key internal-address")
    blog = blog_source([published(version=7)])
    with pytest.raises(RagError) as error:
        await service.sync_article(18, access_token="private-jwt", blog_client=blog)
    assert "secret-key" not in str(error.value) and "internal-address" not in str(error.value)
    cloud.client.delete.assert_not_called()
    assert published_point_id(18, 5, 0) in cloud.points
    getattr(cloud.client, stage).side_effect = original
    await service.sync_article(18, access_token="private-jwt", blog_client=blog)
    assert {p["version_no"] for p in cloud.points.values()} == {7}
    assert all(call[0] == "get_published_knowledge" for call in blog.mock_calls)


@pytest.mark.anyio
async def test_payload_verification_failure_never_deletes_old_version(cloud, service):
    seed(cloud, published())
    cloud.client.retrieve.side_effect = None
    cloud.client.retrieve.return_value = []
    with pytest.raises(RagError, match="verification"):
        await service.sync_article(18, access_token="private-jwt", blog_client=blog_source([published(version=7)]))
    cloud.client.delete.assert_not_called()


@pytest.mark.anyio
async def test_reconcile_repairs_missing_stale_archived_and_deleted_articles(cloud, service):
    seed(cloud, published(version=1))
    seed(cloud, published(version=3))
    seed(cloud, published(article=20))
    seed(cloud, published(article=21))
    current = [published(version=7), published(article=19)]
    await service.reconcile(access_token="private-jwt", blog_client=blog_source(current))
    expected = {published_point_id(r.article_id, r.published_version, c.index) for r in current for c in r.chunks}
    assert set(cloud.points) == expected
    await service.sync_article(18, access_token="private-jwt", blog_client=blog_source([]))
    assert {p["article_id"] for p in cloud.points.values()} == {19}


@pytest.mark.anyio
async def test_sitewide_ranking_diversity_and_fresh_canonical_verification(cloud, service):
    a, b = published(), published(article=19)
    old = published(version=1)
    archived = published(article=20)
    rows = []
    for score, record, index in [(.99, archived, 0), (.98, old, 0), (.97, a, 0), (.96, a, 1), (.95, a, 2), (.94, b, 2)]:
        rows.append(models.ScoredPoint(id=published_point_id(record.article_id, record.published_version, index),
            version=0, score=score, payload=published_payload(record, record.chunks[index])))
    cloud.client.query_points.return_value = QueryResponse(points=rows)
    blog = blog_source([a, b])
    hits = await service.search_published_knowledge("缓存", access_token="private-jwt", blog_client=blog)
    assert [(h.article_id, h.chunk_index, h.score) for h in hits] == [(18, 0, .97), (18, 1, .96), (19, 2, .94)]
    assert blog.get_published_knowledge.await_count == 2
    args = cloud.client.query_points.call_args.kwargs
    assert args["query"] == models.Document(text="缓存", model="intfloat/multilingual-e5-small")
    assert args["limit"] == 9 and not args["with_vectors"]
    # 查询期间归档/重新发布的所有旧命中也会被排除。
    blog.get_published_knowledge.side_effect = [[a, b], []]
    assert await service.search_published_knowledge("缓存", access_token="private-jwt", blog_client=blog) == []


@pytest.mark.anyio
@pytest.mark.parametrize("field", ["text", "heading_path", "title"])
async def test_mismatched_payload_never_becomes_evidence(cloud, service, field):
    record = published()
    payload = published_payload(record, record.chunks[0])
    payload[field] = [] if field == "heading_path" else "wrong"
    cloud.client.query_points.return_value = QueryResponse(points=[models.ScoredPoint(id=published_point_id(18, 5, 0), version=0, score=.9, payload=payload)])
    assert await service.search_published_knowledge("q", access_token="private-jwt", blog_client=blog_source([record])) == []


@pytest.mark.anyio
async def test_auth_failure_before_any_index_or_inference(cloud, service):
    blog = Mock(get_published_knowledge=AsyncMock(side_effect=AuthenticationError("bad", method="GET", path="/source")))
    for operation in [service.sync_article(18, access_token="bad", blog_client=blog),
                      service.reconcile(access_token="bad", blog_client=blog),
                      service.search_published_knowledge("q", access_token="bad", blog_client=blog)]:
        with pytest.raises(AuthenticationError):
            await operation
    assert cloud.client.mock_calls == []


@pytest.mark.anyio
async def test_client_mapping_jwt_and_invalid_source_contract():
    data = published().model_dump(mode="json")
    requests = []
    def handle(request):
        requests.append(request)
        return httpx.Response(200, json={"code": 200, "data": [data]})
    async with BlogClient(Settings(_env_file=None), transport=httpx.MockTransport(handle)) as blog:
        assert await blog.get_published_knowledge(access_token="private-jwt") == [published()]
        await blog.get_published_knowledge(article_id=18, access_token="other-jwt")
        assert requests[0].url.path == "/api/v1/agent/published-knowledge"
        assert requests[1].url.path.endswith("/18")
        assert [r.headers["Authorization"] for r in requests] == ["Bearer private-jwt", "Bearer other-jwt"]
        with pytest.raises(BlogBackendError): await blog.get_published_knowledge(article_id=19, access_token="private-jwt")
        data["chunks"][0]["index"] = 99
        with pytest.raises(BlogBackendError): await blog.get_published_knowledge(access_token="private-jwt")


@pytest.mark.anyio
async def test_grounded_answer_sources_are_server_owned_and_no_evidence_skips_model():
    model_factory = Mock(side_effect=AssertionError("no general knowledge fallback"))
    no = await grounded_answer("q", [], model_factory=model_factory, scope="本站当前已发布文章中")
    assert not no.has_evidence and no.sources == [] and "没有足够检索依据" in no.answer
    model_factory.assert_not_called()
    record = published()
    hit = KnowledgeHit(**published_payload(record, record.chunks[0]), score=.9)
    model = Mock(ainvoke=AsyncMock(return_value=AIMessage(content="文章 18 V5 的证据。")))
    answer = await grounded_answer("q", [hit], model_factory=lambda: model, scope="本站当前已发布文章中")
    assert answer.sources[0].model_dump() == {k: v for k, v in hit.model_dump().items() if k not in {"score", "text"}}
    messages = model.ainvoke.call_args.args[0]
    assert json.loads(messages[-1].content)["evidence"][0]["text"] == hit.text


@pytest.mark.anyio
async def test_published_factory_uses_distinct_cloud_profile_and_no_local_model(monkeypatch):
    settings = cloud_settings(app_env="production", published_knowledge_collection="published_knowledge_e5_cloud_v1")
    monkeypatch.setattr(factory, "get_settings", lambda: settings)
    client = Mock(close=AsyncMock())
    constructor = Mock(return_value=client)
    monkeypatch.setattr(factory, "AsyncQdrantClient", constructor)
    local = Mock(side_effect=AssertionError("no local model"))
    monkeypatch.setattr(factory, "get_embedding_provider", local)
    factory.get_published_service.cache_clear()
    try:
        service = factory.get_published_service()
        assert service is factory.get_published_service()
        assert service.store.collection == service.store.profile["collection"] == "published_knowledge_e5_cloud_v1"
        assert service.store.collection != settings.qdrant_collection
        assert constructor.call_args.kwargs["cloud_inference"] is True
        local.assert_not_called()
    finally:
        await factory.close_published_service()


@pytest.fixture
def api(monkeypatch):
    settings = Settings(_env_file=None)
    status = [200]
    def transport(request):
        return httpx.Response(status[0], json={"code": status[0], "data": {"id": 7}, "message": "secret-details"})
    monkeypatch.setattr(knowledge, "get_settings", lambda: settings)
    monkeypatch.setattr(knowledge, "limit_user", Mock())
    monkeypatch.setattr(knowledge, "BlogClient", lambda _: BlogClient(settings, transport=httpx.MockTransport(transport)))
    service = Mock(search_published_knowledge=AsyncMock(return_value=[]), sync_article=AsyncMock(return_value=0))
    constructor = Mock(return_value=service)
    monkeypatch.setattr(knowledge, "get_published_service", constructor)
    model = Mock(side_effect=AssertionError("no evidence must not call model"))
    monkeypatch.setattr(knowledge, "create_model", model)
    app = FastAPI()
    app.include_router(knowledge.router)
    return TestClient(app), service, constructor, status


def test_knowledge_api_login_scope_no_evidence_and_safe_sync_failure(api):
    client, service, constructor, status = api
    path, headers = "/api/v1/agent/knowledge", {"Authorization": "Bearer private-jwt"}
    assert client.post(path+"/chat", json={"query": "q"}).status_code == 401
    constructor.assert_not_called()
    response = client.post(path+"/chat", headers=headers, json={"query": "q"})
    assert response.status_code == 200 and response.json()["has_evidence"] is False
    assert service.search_published_knowledge.call_args.kwargs["access_token"] == "private-jwt"
    for field in ("user_id", "article_id", "version_no", "conversation_id", "top_k"):
        assert client.post(path+"/chat", headers=headers, json={"query": "q", field: 1}).status_code == 422
    service.sync_article.side_effect = RagError("safe error")
    assert client.post(path+"/sync/18", headers=headers).status_code == 502
    for code in (401, 403, 500):
        status[0] = code
        constructor.reset_mock()
        response = client.post(path+"/chat", headers=headers, json={"query": "q"})
        assert response.status_code == (502 if code == 500 else code)
        assert "secret-details" not in response.text
        constructor.assert_not_called()


@pytest.mark.anyio
@pytest.mark.filterwarnings("ignore:Payload indexes have no effect in the local Qdrant:UserWarning")
async def test_local_published_backend_and_login_independent_retrieval_scope():
    settings = Settings(_env_file=None)
    profile = settings.model_copy(update={"qdrant_collection": settings.published_knowledge_collection})
    store = QdrantChunkStore(AsyncQdrantClient(":memory:"), profile, index_fields=("article_id", "version_no"))
    service = PublishedKnowledgeService(store, embedding=FakeEmbedding(settings), top_k=4)
    data = [published(), published(article=19)]
    blog = blog_source(data)
    try:
        await service.reconcile(access_token="private-jwt", blog_client=blog)
        # 登录身份只作为 Go 请求凭据；不进入 Qdrant 过滤器、payload 或检索结果。
        blog.get_published_knowledge.side_effect = None
        blog.get_published_knowledge.return_value = data
        first = await service.search_published_knowledge("q", access_token="user-a-jwt", blog_client=blog)
        second = await service.search_published_knowledge("q", access_token="user-b-jwt", blog_client=blog)
        assert first == second and {h.article_id for h in first} == {18, 19}
        points, _ = await store.client.scroll(store.collection, limit=100)
        assert len(points) == 6 and all("user_id" not in p.payload for p in points)
        serialized = json.dumps([h.model_dump(mode="json") for h in first])
        assert "jwt" not in serialized and "user_id" not in serialized
        assert {call.kwargs["access_token"] for call in blog.get_published_knowledge.call_args_list} >= {"user-a-jwt", "user-b-jwt"}
    finally:
        await service.aclose()

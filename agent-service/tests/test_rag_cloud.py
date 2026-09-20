import copy
from argparse import Namespace
from types import SimpleNamespace
from unittest.mock import AsyncMock, Mock
from uuid import UUID, NAMESPACE_URL

import httpx
import pytest
from pydantic import ValidationError
from qdrant_client import AsyncQdrantClient, models
from qdrant_client.http.models import QueryResponse

from app.clients.errors import AuthenticationError, AuthorizationError, ArticleNotFoundError
from app.core.config import Settings
from app.core import rag as factory
from app.rag import __main__ as cli
from app.rag.backend import QdrantCloudRetrievalBackend
from app.rag.errors import RagError, RagConfigurationError
from app.rag.service import ArticleRagService
from app.rag.store import QdrantChunkStore, point_id, chunk_payload
from test_rag import source


@pytest.fixture
def anyio_backend():
    return "asyncio"


def cloud_settings(**overrides):
    values = dict(embedding_provider="qdrant_cloud", qdrant_url="https://vector.example.test",
                  qdrant_api_key="fake-cloud-key", qdrant_collection="article_chunks_e5_cloud_v1")
    return Settings(_env_file=None, **(values | overrides))


@pytest.fixture
def cloud():
    settings = cloud_settings()
    points, events = {}, []
    client = Mock(spec=AsyncQdrantClient)
    info = SimpleNamespace(config=SimpleNamespace(
        params=SimpleNamespace(vectors=models.VectorParams(size=384, distance=models.Distance.COSINE)),
        metadata={"embedding_profile": settings.embedding_profile}), payload_schema={})
    client.collection_exists = AsyncMock(return_value=True)
    client.get_collection = AsyncMock(return_value=info)
    client.create_collection = AsyncMock()
    client.create_payload_index = AsyncMock()
    client.close = AsyncMock()

    async def scroll(collection, **kwargs):
        events.append("scroll")
        scope = {f.key: f.match.value for f in kwargs["scroll_filter"].must}
        matching = sorted((id for id, p in points.items() if all(p[k] == v for k, v in scope.items())), key=str)
        start = kwargs["offset"] or 0
        # 故意让每页小于请求 limit，验证调用方确实依赖 next_offset。
        end = start + min(2, kwargs["limit"])
        return ([models.Record(id=id, payload=copy.deepcopy(points[id])) for id in matching[start:end]],
                end if end < len(matching) else None)

    async def upsert(collection, **kwargs):
        events.append("upsert")
        for point in kwargs["points"]:
            points[point.id] = copy.deepcopy(point.payload)

    async def retrieve(collection, **kwargs):
        events.append("verify")
        return [models.Record(id=id, payload=copy.deepcopy(points[id])) for id in kwargs["ids"] if id in points]

    async def delete(collection, **kwargs):
        events.append("delete")
        assert isinstance(kwargs["points_selector"], models.PointIdsList)
        for id in kwargs["points_selector"].points:
            points.pop(id, None)

    client.scroll = AsyncMock(side_effect=scroll)
    client.upsert = AsyncMock(side_effect=upsert)
    client.retrieve = AsyncMock(side_effect=retrieve)
    client.delete = AsyncMock(side_effect=delete)
    client.query_points = AsyncMock(return_value=QueryResponse(points=[]))
    backend = QdrantCloudRetrievalBackend(QdrantChunkStore(client, settings))
    return SimpleNamespace(settings=settings, client=client, points=points, events=events,
                           backend=backend, service=ArticleRagService(backend, top_k=2), info=info)


def seed(cloud, data):
    for chunk in data.chunks:
        cloud.points[point_id(data.article_id, data.version_no, chunk.index)] = chunk_payload(data, chunk)


@pytest.mark.anyio
async def test_raw_documents_pagination_verification_and_exact_stale_delete(cloud):
    seed(cloud, source(7))
    seed(cloud, source(2, version=2))
    seed(cloud, source(2, article=19, user=8))
    data = source(3)
    await cloud.backend.replace(data)
    assert cloud.events == ["scroll"] * 4 + ["upsert", "verify", "delete"]
    args = cloud.client.upsert.call_args.kwargs
    assert args["wait"] is True
    for point, chunk in zip(args["points"], data.chunks, strict=True):
        assert isinstance(point.vector, models.Document)
        assert point.vector.text == chunk.text and not point.vector.text.startswith("passage: ")
        assert point.vector.model == "intfloat/multilingual-e5-small"
        assert point.payload == chunk_payload(data, chunk)
        assert point.id == point_id(18, 1, chunk.index)
    assert all(call.kwargs["with_vectors"] is False for call in cloud.client.scroll.call_args_list)
    assert all({f.key: f.match.value for f in call.kwargs["scroll_filter"].must} == {"user_id": 7, "article_id": 18, "version_no": 1}
               for call in cloud.client.scroll.call_args_list)
    assert cloud.client.retrieve.call_args.kwargs["with_payload"] is True
    assert cloud.client.retrieve.call_args.kwargs["with_vectors"] is False
    deletion = cloud.client.delete.call_args.kwargs
    assert deletion["wait"] is True
    assert set(deletion["points_selector"].points) == {point_id(18, 1, i) for i in range(3, 7)}
    assert len(cloud.points) == 7
    previous = copy.deepcopy(cloud.points)
    cloud.client.delete.reset_mock()
    await cloud.backend.replace(data)
    assert cloud.points == previous
    cloud.client.delete.assert_not_called()


@pytest.mark.anyio
async def test_empty_chunks_clear_only_existing_scope_without_inference(cloud):
    seed(cloud, source(3))
    seed(cloud, source(1, version=2))
    await cloud.backend.replace(source(0))
    cloud.client.upsert.assert_not_called()
    cloud.client.retrieve.assert_not_called()
    assert set(cloud.points) == {point_id(18, 2, 0)}
    assert cloud.events == ["scroll", "scroll", "delete"]


@pytest.mark.anyio
@pytest.mark.parametrize("stage", ["scroll", "upsert", "retrieve"])
async def test_sdk_errors_are_safe_and_never_delete_stale(cloud, stage):
    seed(cloud, source(4))
    previous = copy.deepcopy(cloud.points)
    getattr(cloud.client, stage).side_effect = RuntimeError("fake-cloud-key private-network-details")
    with pytest.raises(RagError) as error:
        await cloud.backend.replace(source(2))
    assert "fake-cloud-key" not in str(error.value) and "private-network-details" not in str(error.value)
    assert error.value.__suppress_context__
    cloud.client.delete.assert_not_called()
    assert set(cloud.points) == set(previous)
    if stage != "retrieve":
        assert previous == cloud.points


@pytest.mark.anyio
@pytest.mark.parametrize("fault", ["missing", "duplicate", "unexpected_id", "user_id", "article_id", "version_no", "chunk_index", "heading_path", "text"])
async def test_verification_failure_preserves_stale_points(cloud, fault):
    seed(cloud, source(4))
    original = cloud.client.retrieve.side_effect

    async def corrupt(collection, **kwargs):
        records = await original(collection, **kwargs)
        if fault == "missing": return records[:-1]
        if fault == "duplicate": return [records[0], records[0]]
        if fault == "unexpected_id": records[0].id = point_id(19, 1, 0)
        elif fault == "heading_path": records[0].payload[fault] = []
        elif fault == "text": records[0].payload[fault] = "stale text"
        else: records[0].payload[fault] = 99
        return records

    cloud.client.retrieve.side_effect = corrupt
    with pytest.raises(RagError, match="verification"):
        await cloud.backend.replace(source(2))
    cloud.client.delete.assert_not_called()
    assert point_id(18, 1, 3) in cloud.points


@pytest.mark.anyio
async def test_all_batches_verified_before_any_delete_and_explicit_retry_recovers(cloud):
    cloud.backend._batch_size = 2
    seed(cloud, source(7))
    original = cloud.client.upsert.side_effect

    async def fail_second(collection, **kwargs):
        if cloud.client.upsert.await_count == 2: raise RuntimeError("inference failure")
        await original(collection, **kwargs)

    cloud.client.upsert.side_effect = fail_second
    with pytest.raises(RagError): await cloud.backend.replace(source(5))
    cloud.client.retrieve.assert_not_called()
    cloud.client.delete.assert_not_called()
    cloud.client.upsert.side_effect = original
    cloud.events.clear()
    await cloud.backend.replace(source(5))
    assert cloud.events == ["scroll"] * 4 + ["upsert"] * 3 + ["verify"] * 3 + ["delete"]
    assert len(cloud.points) == 5


@pytest.mark.anyio
async def test_later_verification_batch_failure_prevents_all_stale_deletion(cloud):
    cloud.backend._batch_size = 2
    seed(cloud, source(7))
    original = cloud.client.retrieve.side_effect

    async def missing_second_batch(collection, **kwargs):
        records = await original(collection, **kwargs)
        return [] if cloud.client.retrieve.await_count == 2 else records

    cloud.client.retrieve.side_effect = missing_second_batch
    with pytest.raises(RagError, match="verification"):
        await cloud.backend.replace(source(5))
    assert cloud.client.upsert.await_count == 3
    assert cloud.client.retrieve.await_count == 2
    cloud.client.delete.assert_not_called()
    assert len(cloud.points) == 7


@pytest.mark.anyio
@pytest.mark.parametrize("fault", ["scope", "pagination"])
async def test_invalid_scroll_stops_before_inference_or_deletion(cloud, fault):
    data = source(1, user=99 if fault == "scope" else 7)
    record = models.Record(id=point_id(18, 1, 0), payload=chunk_payload(data, data.chunks[0]))
    cloud.client.scroll.side_effect = None
    cloud.client.scroll.return_value = ([record], 1 if fault == "pagination" else None)
    with pytest.raises(RagError, match="scope|pagination"):
        await cloud.backend.replace(source(1))
    assert cloud.client.scroll.await_count == (2 if fault == "pagination" else 1)
    cloud.client.upsert.assert_not_called()
    cloud.client.delete.assert_not_called()


@pytest.mark.anyio
async def test_delete_failure_is_safe_without_automatic_retry(cloud):
    seed(cloud, source(4))
    original = cloud.client.delete.side_effect
    cloud.client.delete.side_effect = RuntimeError("fake-cloud-key private-network-details")
    with pytest.raises(RagError, match="explicit retry") as error:
        await cloud.backend.replace(source(2))
    assert "fake-cloud-key" not in str(error.value) and "private-network-details" not in str(error.value)
    cloud.client.delete.assert_awaited_once()
    assert len(cloud.points) == 4
    cloud.client.delete.side_effect = original
    await cloud.backend.replace(source(2))
    assert len(cloud.points) == 2


@pytest.mark.anyio
@pytest.mark.parametrize("mismatch", ["dimension", "distance", "model", "provider", "missing"])
async def test_cloud_profile_mismatch_refuses_write_delete_and_query(cloud, mismatch):
    if mismatch == "dimension": cloud.info.config.params.vectors.size = 768
    elif mismatch == "distance": cloud.info.config.params.vectors.distance = models.Distance.DOT
    elif mismatch == "missing": cloud.info.config.metadata = {}
    else: cloud.info.config.metadata["embedding_profile"][mismatch] = "wrong-profile"
    with pytest.raises(RagConfigurationError): await cloud.backend.replace(source(1))
    with pytest.raises(RagConfigurationError): await cloud.backend.search(source(1), "问题", 3)
    for name in ("scroll", "upsert", "retrieve", "delete", "query_points", "create_collection"):
        getattr(cloud.client, name).assert_not_called()


@pytest.mark.anyio
async def test_cloud_creates_profile_and_integer_payload_indexes_only_on_index(cloud):
    cloud.client.collection_exists.return_value = False
    assert await cloud.backend.search(source(1), "问题", 5) == []
    cloud.client.create_collection.assert_not_called()
    cloud.client.query_points.assert_not_called()
    await cloud.backend.replace(source(1))
    kwargs = cloud.client.create_collection.call_args.kwargs
    assert kwargs["vectors_config"] == models.VectorParams(size=384, distance=models.Distance.COSINE)
    assert kwargs["metadata"] == {"embedding_profile": cloud.settings.embedding_profile}
    assert {c.kwargs["field_name"] for c in cloud.client.create_payload_index.call_args_list} == {"user_id", "article_id", "version_no"}
    assert all(c.kwargs["field_schema"] == models.PayloadSchemaType.INTEGER and c.kwargs["wait"] for c in cloud.client.create_payload_index.call_args_list)


@pytest.mark.anyio
@pytest.mark.parametrize("error_type", [AuthenticationError, AuthorizationError, ArticleNotFoundError])
async def test_go_rejection_precedes_all_cloud_calls(cloud, error_type):
    blog = Mock(get_version_chunks=AsyncMock(side_effect=error_type("rejected", method="GET", path="/chunks", status_code=403)))
    with pytest.raises(error_type): await cloud.service.index_article_version(18, 1, access_token="token", blog_client=blog)
    with pytest.raises(error_type): await cloud.service.search_article_version(18, 1, "问题", access_token="token", blog_client=blog)
    assert cloud.client.mock_calls == []


@pytest.mark.anyio
async def test_raw_query_overfetch_ranking_canonical_checks_and_deduplication(cloud):
    data = source(4)
    rows = []
    for index, score in [(20, .99), (0, .98), (1, .97), (3, .95), (3, .94), (2, .9), (0, .8)]:
        payload = chunk_payload(data, data.chunks[min(index, 3)]) | {"chunk_index": index}
        if score == .98: payload["text"] = "stale-private-text"
        if score == .97: payload["heading_path"] = []
        rows.append(models.ScoredPoint(id=point_id(18, 1, index), version=0, score=score, payload=payload))
    cloud.client.query_points.return_value = QueryResponse(points=rows)
    blog = Mock(get_version_chunks=AsyncMock(return_value=data))
    hits = await cloud.service.search_article_version(18, 1, "Redis 怎样工作？", access_token="token", blog_client=blog)
    assert [(h.chunk_index, h.score) for h in hits] == [(3, .95), (2, .9)]
    kwargs = cloud.client.query_points.call_args.kwargs
    assert kwargs["query"] == models.Document(text="Redis 怎样工作？", model="intfloat/multilingual-e5-small")
    assert kwargs["limit"] == 6 and kwargs["with_payload"] is True and kwargs["with_vectors"] is False
    assert {f.key: f.match.value for f in kwargs["query_filter"].must} == {"user_id": 7, "article_id": 18, "version_no": 1}
    cloud.service.top_k = 50
    await cloud.service.search_article_version(18, 1, "q", access_token="token", blog_client=blog)
    assert cloud.client.query_points.call_args.kwargs["limit"] == 100


@pytest.mark.anyio
@pytest.mark.parametrize("field", ["user_id", "article_id", "version_no", "id"])
async def test_cloud_search_scope_and_point_identity_fail_closed(cloud, field):
    data = source(1)
    payload = chunk_payload(data, data.chunks[0])
    if field != "id": payload[field] = 99
    cloud.client.query_points.return_value = QueryResponse(points=[models.ScoredPoint(
        id=point_id(99 if field == "id" else 18, 1, 0), version=0, score=.9, payload=payload)])
    with pytest.raises(RagError): await cloud.backend.search(data, "问题", 3)


@pytest.mark.anyio
async def test_cloud_query_error_is_safe_and_empty_query_avoids_go_and_cloud(cloud):
    cloud.client.query_points.side_effect = RuntimeError("fake-cloud-key private-network-error")
    with pytest.raises(RagError) as error: await cloud.backend.search(source(1), "问题", 3)
    assert "fake-cloud-key" not in str(error.value) and "private-network-error" not in str(error.value)
    blog = Mock()
    with pytest.raises(ValueError): await cloud.service.search_article_version(18, 1, " ", access_token="token", blog_client=blog)
    assert blog.mock_calls == []


@pytest.mark.parametrize("overrides", [
    {"embedding_model": "wrong"}, {"embedding_dimension": 768}, {"qdrant_distance": "Dot"},
    {"qdrant_api_key": ""}, {"qdrant_api_key": "  "}, {"qdrant_url": "http://vector.example.test"},
])
def test_cloud_configuration_rejected_without_secret_leak(overrides):
    with pytest.raises(ValidationError) as error: cloud_settings(**overrides)
    assert "fake-cloud-key" not in str(error.value)


@pytest.mark.anyio
@pytest.mark.parametrize("environment", ["development", "production"])
async def test_cloud_factory_cached_no_local_model_and_warm_rejected(monkeypatch, environment):
    settings = cloud_settings(app_env=environment)
    monkeypatch.setattr(factory, "get_settings", lambda: settings)
    local = Mock(side_effect=AssertionError("must not create a local model"))
    monkeypatch.setattr(factory, "LocalE5EmbeddingProvider", local)
    client = Mock(close=AsyncMock())
    constructor = Mock(return_value=client)
    monkeypatch.setattr(factory, "AsyncQdrantClient", constructor)
    factory.get_rag_service.cache_clear()
    factory.get_embedding_provider.cache_clear()
    try:
        first = factory.get_rag_service()
        assert first is factory.get_rag_service()
        assert isinstance(first.backend, QdrantCloudRetrievalBackend)
        constructor.assert_called_once_with(url=str(settings.qdrant_url), timeout=10, api_key="fake-cloud-key", cloud_inference=True, check_compatibility=False)
        with pytest.raises(RagConfigurationError, match="warm-model"):
            await cli.run(Namespace(command="warm-model"))
        local.assert_not_called()
        await factory.close_rag_service()
        client.close.assert_awaited_once()
        assert factory.get_rag_service.cache_info().currsize == 0
    finally:
        factory.get_rag_service.cache_clear()
        factory.get_embedding_provider.cache_clear()


def test_stable_point_namespace_and_profile_isolation():
    import uuid
    expected = str(uuid.uuid5(NAMESPACE_URL, "blog-system/article/18/version/1/chunk/2"))
    assert point_id(18, 1, 2) == expected and UUID(expected).version == 5
    assert point_id(1, 23, 4) != point_id(12, 3, 4)
    local = Settings(_env_file=None)
    assert local.embedding_profile != cloud_settings(qdrant_collection=local.qdrant_collection).embedding_profile


@pytest.mark.parametrize("provider", ["local_e5", "remote"])
def test_production_factory_fails_before_creating_clients_or_local_model(monkeypatch, provider):
    settings = Settings(_env_file=None, app_env="production", embedding_provider=provider)
    monkeypatch.setattr(factory, "get_settings", lambda: settings)
    constructor = Mock(side_effect=AssertionError("must not construct a client"))
    local = Mock(side_effect=AssertionError("must not construct a local provider"))
    monkeypatch.setattr(factory, "AsyncQdrantClient", constructor)
    monkeypatch.setattr(factory, "LocalE5EmbeddingProvider", local)
    factory.get_rag_service.cache_clear()
    factory.get_embedding_provider.cache_clear()
    try:
        with pytest.raises(RagConfigurationError):
            factory.get_rag_service()
        constructor.assert_not_called()
        local.assert_not_called()
    finally:
        factory.get_rag_service.cache_clear()
        factory.get_embedding_provider.cache_clear()


@pytest.mark.anyio
async def test_real_sdk_serializes_cloud_documents_without_local_inference(monkeypatch):
    import json
    requests = []

    def handler(request):
        requests.append(request)
        if request.url.path.endswith("/query"):
            return httpx.Response(200, json={"result": {"points": []}, "status": "ok", "time": 0})
        return httpx.Response(200, json={"result": {"operation_id": 1, "status": "completed"}, "status": "ok", "time": 0})

    client = AsyncQdrantClient(url="https://vector.example.test", api_key="fake-cloud-key",
                               cloud_inference=True, check_compatibility=False, transport=httpx.MockTransport(handler))
    local_embed = Mock(side_effect=AssertionError("must not run FastEmbed"))
    monkeypatch.setattr(client, "_embed_models", local_embed)
    backend = QdrantCloudRetrievalBackend(QdrantChunkStore(client, cloud_settings()))
    monkeypatch.setattr(backend.store, "ensure_collection", AsyncMock(return_value=True))
    monkeypatch.setattr(backend, "_existing_ids", AsyncMock(return_value=set()))
    data = source(1)
    monkeypatch.setattr(client, "retrieve", AsyncMock(return_value=[models.Record(id=point_id(18, 1, 0), payload=chunk_payload(data, data.chunks[0]))]))
    try:
        await backend.replace(data)
        await backend.search(data, "原始问题", 3)
        assert len(requests) == 2
        indexed = json.loads(requests[0].content)["points"][0]["vector"]
        query = json.loads(requests[1].content)["query"]["nearest"]
        assert indexed["text"] == data.chunks[0].text and query["text"] == "原始问题"
        assert indexed["model"] == query["model"] == "intfloat/multilingual-e5-small"
        assert requests[0].url.params["wait"] == "true"
        local_embed.assert_not_called()
    finally:
        await backend.aclose()

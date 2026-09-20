from unittest.mock import AsyncMock, Mock

import pytest
from qdrant_client import AsyncQdrantClient, models
from qdrant_client.http.models import QueryResponse

from app.clients.errors import ArticleNotFoundError, AuthenticationError, AuthorizationError
from app.clients.models import ArticleVersionChunks
from app.core.config import Settings
from app.rag.errors import EmbeddingError, RagConfigurationError, RagError
from app.rag.models import RetrievedChunk, assemble_context
from app.rag.backend import LocalRetrievalBackend
from app.rag.service import ArticleRagService
from app.rag.store import QdrantChunkStore, point_id


@pytest.fixture
def anyio_backend():
    return "asyncio"


def source(count=4, *, article=18, version=1, user=7):
    return ArticleVersionChunks.model_validate({
        "article_id": article, "version_no": version, "user_id": user,
        "chunks": [{
            "index": i, "heading_path": [{"level": 1, "text": "Redis"}],
            "blocks": [{"type": "paragraph", "text": f"Text {i}"}], "text": f"Redis\n\nText {i}",
        } for i in range(count)],
    })


class FakeEmbedding:
    def __init__(self, settings):
        self.dimension = settings.embedding_dimension
        self.profile = settings.embedding_profile

    def embed_documents(self, texts):
        return [[float(i + 1), 1.] + [0.] * 382 for i in range(len(texts))]

    def embed_query(self, text):
        return [1., 0.] + [0.] * 382


@pytest.fixture
async def rag():
    settings = Settings(_env_file=None)
    client = AsyncQdrantClient(":memory:")
    store = QdrantChunkStore(client, settings)
    service = ArticleRagService(LocalRetrievalBackend(FakeEmbedding(settings), store), top_k=settings.rag_top_k)
    yield service
    await store.aclose()


@pytest.mark.anyio
@pytest.mark.filterwarnings("ignore:Payload indexes have no effect in the local Qdrant:UserWarning")
async def test_index_points_scope_order_replacement_and_explicit_retry(rag):
    blog = Mock(get_version_chunks=AsyncMock(return_value=source(7)))
    for data in [source(7), source(2, version=2), source(2, article=19, user=8)]:
        blog.get_version_chunks.return_value = data
        await rag.index_article_version(data.article_id, data.version_no, access_token="token", blog_client=blog)
    blog.get_version_chunks.return_value = source(7)
    hits = await rag.search_article_version(18, 1, "question", access_token="token", blog_client=blog)
    assert len(hits) == 5
    assert all(type(hit) is RetrievedChunk and hit.article_id == 18 and hit.version_no == 1 for hit in hits)
    assert [hit.chunk_index for hit in hits] == [6, 5, 4, 3, 2]
    context = assemble_context(hits)
    assert context.index("Chunk 6") < context.index("Chunk 5")
    assert "passage: " not in context
    # 同一版本重建两次，再缩减块数；不产生重复或残留。
    for count in (7, 4, 3, 3):
        blog.get_version_chunks.return_value = source(count)
        await rag.index_article_version(18, 1, access_token="token", blog_client=blog)
    records, _ = await rag.backend.store.client.scroll(rag.backend.store.collection, limit=100)
    owned = [point for point in records if point.payload["article_id"] == 18 and point.payload["version_no"] == 1]
    assert len(owned) == 3 and len(records) == 7
    assert {str(point.id) for point in owned} == {point_id(18, 1, i) for i in range(3)}
    for point in owned:
        payload = point.payload
        chunk = source(3).chunks[payload["chunk_index"]]
        assert payload == {
            "user_id": 7, "article_id": 18, "version_no": 1, "chunk_index": chunk.index,
            "heading_path": [{"level": 1, "text": "Redis"}], "text": chunk.text,
        }
    assert await rag.backend.store.search(8, 18, 1, rag.backend.embedding.embed_query("q"), 5) == []
    blog.get_version_chunks.return_value = source(0)
    await rag.index_article_version(18, 1, access_token="token", blog_client=blog)
    assert await rag.backend.store.search(7, 18, 1, rag.backend.embedding.embed_query("q"), 5) == []
    assert (await rag.backend.store.client.count(rag.backend.store.collection)).count == 4


@pytest.mark.anyio
@pytest.mark.filterwarnings("ignore:Payload indexes have no effect in the local Qdrant:UserWarning")
@pytest.mark.parametrize("failure", ["exception", "dimension", "count", "nan"])
async def test_embedding_failure_preserves_existing_points(rag, monkeypatch, failure):
    blog = Mock(get_version_chunks=AsyncMock(return_value=source(2)))
    await rag.index_article_version(18, 1, access_token="token", blog_client=blog)
    previous, _ = await rag.backend.store.client.scroll(rag.backend.store.collection, with_vectors=True)
    embed = Mock(side_effect=RuntimeError("secret-details")) if failure == "exception" else Mock(return_value={
        "dimension": [[1.], [1.]], "count": [], "nan": [[float("nan")] * 384] * 2,
    }[failure])
    monkeypatch.setattr(rag.backend.embedding, "embed_documents", embed)
    delete = AsyncMock(wraps=rag.backend.store.client.delete)
    monkeypatch.setattr(rag.backend.store.client, "delete", delete)
    with pytest.raises(RagError):
        await rag.index_article_version(18, 1, access_token="token", blog_client=blog)
    delete.assert_not_called()
    after, _ = await rag.backend.store.client.scroll(rag.backend.store.collection, with_vectors=True)
    assert previous == after


@pytest.mark.anyio
@pytest.mark.filterwarnings("ignore:Payload indexes have no effect in the local Qdrant:UserWarning")
async def test_upsert_failure_can_be_repaired_by_explicit_reindex(rag, monkeypatch):
    blog = Mock(get_version_chunks=AsyncMock(return_value=source(3)))
    upsert = rag.backend.store.client.upsert
    broken = AsyncMock(side_effect=RuntimeError("fake-secret"))
    monkeypatch.setattr(rag.backend.store.client, "upsert", broken)
    with pytest.raises(RagError, match="explicit retry") as error:
        await rag.index_article_version(18, 1, access_token="token", blog_client=blog)
    assert "fake-secret" not in str(error.value)
    assert broken.await_count == 1
    monkeypatch.setattr(rag.backend.store.client, "upsert", upsert)
    await rag.index_article_version(18, 1, access_token="token", blog_client=blog)
    assert (await rag.backend.store.client.count(rag.backend.store.collection)).count == 3


@pytest.mark.anyio
@pytest.mark.parametrize("error_type", [AuthenticationError, AuthorizationError, ArticleNotFoundError])
async def test_go_authorizes_before_embedding_or_qdrant(rag, monkeypatch, error_type):
    blog = Mock(get_version_chunks=AsyncMock(side_effect=error_type("rejected", method="GET", path="/chunks", status_code=403)))
    docs, query, replace, search = Mock(), Mock(), AsyncMock(), AsyncMock()
    monkeypatch.setattr(rag.backend.embedding, "embed_documents", docs)
    monkeypatch.setattr(rag.backend.embedding, "embed_query", query)
    monkeypatch.setattr(rag.backend.store, "replace", replace)
    monkeypatch.setattr(rag.backend.store, "search", search)
    with pytest.raises(error_type):
        await rag.index_article_version(18, 1, access_token="token", blog_client=blog)
    with pytest.raises(error_type):
        await rag.search_article_version(18, 1, "question", access_token="token", blog_client=blog)
    for call in (docs, query, replace, search):
        call.assert_not_called()


@pytest.mark.anyio
async def test_filters_top_k_domain_mapping_and_stale_content_rejection(rag, monkeypatch):
    blog = Mock(get_version_chunks=AsyncMock(return_value=source(3)))
    points = [models.ScoredPoint(id=point_id(18, 1, i), version=0, score=score, payload={
        "user_id": 7, "article_id": 18, "version_no": 1, "chunk_index": i,
        "heading_path": [{"level": 1, "text": "Redis"}], "text": text,
    }) for i, score, text in [(2, .9, source(3).chunks[2].text), (0, .8, "stale-private-text"), (1, .7, source(3).chunks[1].text)]]
    monkeypatch.setattr(rag.backend.store, "ensure_collection", AsyncMock(return_value=True))
    query = AsyncMock(return_value=QueryResponse(points=points))
    monkeypatch.setattr(rag.backend.store.client, "query_points", query)
    rag.top_k = 3
    hits = await rag.search_article_version(18, 1, "q", access_token="token", blog_client=blog)
    assert [hit.chunk_index for hit in hits] == [2, 1]
    assert "stale-private-text" not in assemble_context(hits)
    kwargs = query.call_args.kwargs
    assert kwargs["limit"] == 9 and kwargs["with_vectors"] is False
    assert {field.key: field.match.value for field in kwargs["query_filter"].must} == {"user_id": 7, "article_id": 18, "version_no": 1}
    points[0].payload["user_id"] = 99
    with pytest.raises(RagError, match="scope"):
        await rag.search_article_version(18, 1, "q", access_token="token", blog_client=blog)


@pytest.mark.anyio
@pytest.mark.parametrize("mismatch", ["dimension", "distance", "model", "provider", "missing"])
async def test_incompatible_collection_rejected_before_delete_or_query(rag, monkeypatch, mismatch):
    profile = dict(rag.backend.store.profile)
    if mismatch in ("model", "provider"):
        profile[mismatch] = "different"
    await rag.backend.store.client.create_collection(
        rag.backend.store.collection,
        vectors_config=models.VectorParams(size=768 if mismatch == "dimension" else 384, distance=models.Distance.DOT if mismatch == "distance" else models.Distance.COSINE),
        metadata={} if mismatch == "missing" else {"embedding_profile": profile},
    )
    delete, query = AsyncMock(), AsyncMock()
    monkeypatch.setattr(rag.backend.store.client, "delete", delete)
    monkeypatch.setattr(rag.backend.store.client, "query_points", query)
    with pytest.raises(RagConfigurationError):
        await rag.backend.store.replace(source(1), rag.backend.embedding.embed_documents(["text"]))
    with pytest.raises(RagConfigurationError):
        await rag.backend.store.search(7, 18, 1, rag.backend.embedding.embed_query("q"), 5)
    delete.assert_not_called()
    query.assert_not_called()


@pytest.mark.anyio
async def test_read_does_not_create_collection_and_index_creates_payload_indexes(rag, monkeypatch):
    assert await rag.backend.store.search(7, 18, 1, rag.backend.embedding.embed_query("q"), 5) == []
    assert not await rag.backend.store.client.collection_exists(rag.backend.store.collection)
    create_index = AsyncMock()
    monkeypatch.setattr(rag.backend.store.client, "create_payload_index", create_index)
    await rag.backend.store.ensure_collection(create=True)
    assert {call.kwargs["field_name"] for call in create_index.call_args_list} == {"user_id", "article_id", "version_no"}
    assert all(call.kwargs["field_schema"] == models.PayloadSchemaType.INTEGER for call in create_index.call_args_list)


def test_provider_profile_must_match_store():
    settings = Settings(_env_file=None)
    embedding = FakeEmbedding(settings)
    store = Mock(dimension=384, profile={**settings.embedding_profile, "model": "different"})
    with pytest.raises(RagConfigurationError):
        LocalRetrievalBackend(embedding, store)

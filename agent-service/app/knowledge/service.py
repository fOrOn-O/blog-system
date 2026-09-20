import asyncio
from collections import Counter
from uuid import NAMESPACE_URL, uuid5

from qdrant_client import models

from app.clients.blog import BlogClient
from app.clients.models import PublishedKnowledgeRecord
from app.rag.embedding import EmbeddingProvider, validate_vectors
from app.rag.errors import RagError, RagConfigurationError
from app.rag.store import QdrantChunkStore
from app.rag.verified_write import upsert_verify_delete
from app.knowledge.models import KnowledgeHit


def published_point_id(article_id: int, version: int, index: int) -> str:
    return str(uuid5(NAMESPACE_URL, f"blog-system/published/article/{article_id}/version/{version}/chunk/{index}"))


def published_payload(record, chunk):
    return dict(article_id=record.article_id, version_no=record.published_version, title=record.title,
                chunk_index=chunk.index, heading_path=[h.model_dump() for h in chunk.heading_path], text=chunk.text)


class PublishedKnowledgeService:
    def __init__(self, store: QdrantChunkStore, *, embedding: EmbeddingProvider | None = None, top_k: int = 5):
        self.store, self.embedding, self.top_k = store, embedding, top_k
        if embedding is None and store.profile["provider"] != "qdrant_cloud":
            raise RagConfigurationError("Published knowledge requires a configured embedding backend")
        if embedding is not None and any(embedding.profile[k] != store.profile[k] for k in ("provider", "model", "dimension", "distance")):
            raise RagConfigurationError("Published embedding profile mismatch")
        self._lock = asyncio.Lock()

    async def _scroll(self, article_id: int | None = None):
        found, offset, seen = [], None, set()
        scope = None if article_id is None else models.Filter(must=[models.FieldCondition(key="article_id", match=models.MatchValue(value=article_id))])
        while True:
            rows, next_offset = await self.store.client.scroll(self.store.collection, scroll_filter=scope,
                limit=64, offset=offset, with_payload=["article_id"], with_vectors=False)
            for row in rows:
                value = (row.payload or {}).get("article_id")
                if type(value) is not int or value <= 0 or (article_id is not None and value != article_id):
                    raise RagError("Published index scope mismatch")
            found.extend(rows)
            if next_offset is None:
                return found
            if next_offset in seen:
                raise RagError("Published index pagination did not advance")
            seen.add(next_offset)
            offset = next_offset

    async def _replace(self, article_id: int, record: PublishedKnowledgeRecord | None):
        chunks = record.chunks if record else []
        if self.embedding is not None:
            vectors = await asyncio.to_thread(self.embedding.embed_documents, [c.text for c in chunks])
            validate_vectors(vectors, len(chunks), self.store.dimension)
        else:
            vectors = [models.Document(text=c.text, model=str(self.store.profile["model"])) for c in chunks]
        await self.store.ensure_collection(create=True)
        existing = {row.id for row in await self._scroll(article_id)}
        points = [models.PointStruct(id=published_point_id(article_id, record.published_version, c.index),
                    vector=v, payload=published_payload(record, c)) for c, v in zip(chunks, vectors, strict=True)]
        await upsert_verify_delete(self.store.client, self.store.collection, points, existing)

    async def sync_article(self, article_id: int, *, access_token: str, blog_client: BlogClient) -> int:
        # 源查询也在进程锁内；晚到的请求不能携带锁外读取的旧状态覆盖新状态。
        async with self._lock:
            records = await blog_client.get_published_knowledge(article_id=article_id, access_token=access_token)
            try:
                await self._replace(article_id, records[0] if records else None)
            except RagError:
                raise
            except Exception:
                raise RagError("Published sync failed; explicit retry or reconcile is required") from None
            return len(records[0].chunks) if records else 0

    async def reconcile(self, *, access_token: str, blog_client: BlogClient) -> int:
        async with self._lock:
            current = await blog_client.get_published_knowledge(access_token=access_token)
            try:
                await self.store.ensure_collection(create=True)
                ids = {r.article_id for r in current} | {r.payload["article_id"] for r in await self._scroll()}
                for article_id in sorted(ids):
                    # 再次从 Go 读取单篇当前状态；不根据开始时的列表决定删除。
                    records = await blog_client.get_published_knowledge(article_id=article_id, access_token=access_token)
                    await self._replace(article_id, records[0] if records else None)
                return len(ids)
            except RagError:
                raise
            except Exception:
                raise RagError("Published reconcile incomplete; explicit retry is required") from None

    async def search_published_knowledge(self, query: str, *, access_token: str, blog_client: BlogClient) -> list[KnowledgeHit]:
        if not query.strip():
            raise ValueError("Query must not be empty")
        # 凭据只用于 Go API 传输；登录策略不决定检索范围，不添加 ownership/user_id 过滤。
        # 任何 Qdrant/推理之前先获取 Go 的全站公开源。
        current = await blog_client.get_published_knowledge(access_token=access_token)
        if not current:
            return []
        try:
            if not await self.store.ensure_collection(create=False):
                return []
            if self.embedding is None:
                vector = models.Document(text=query, model=str(self.store.profile["model"]))
            else:
                vector = await asyncio.to_thread(self.embedding.embed_query, query)
                validate_vectors([vector], 1, self.store.dimension)
            response = await self.store.client.query_points(self.store.collection, query=vector,
                limit=min(self.top_k * 3, 100), with_payload=True, with_vectors=False)
        except RagError:
            raise
        except Exception:
            raise RagError("Published retrieval unavailable") from None
        # 检索期间可能发布/归档；重新读取 authority，过期结果不得进入上下文。
        latest = await blog_client.get_published_knowledge(access_token=access_token)
        canonical = {(r.article_id, r.published_version, c.index): published_payload(r, c) for r in latest for c in r.chunks}
        candidates, seen = [], set()
        for point in response.points:
            payload = point.payload or {}
            try:
                hit = KnowledgeHit.model_validate({**payload, "score": point.score})
            except ValueError:
                raise RagError("Invalid published retrieval identity") from None
            key = (hit.article_id, hit.version_no, hit.chunk_index)
            if str(point.id) != published_point_id(*key):
                raise RagError("Invalid published point identity")
            if key in seen or canonical.get(key) != payload:
                continue
            seen.add(key)
            candidates.append(hit)
        # 每篇最多两块，保留候选顺序与原始 score；避免单篇淹没跨文章证据。
        counts, result = Counter(), []
        for hit in candidates:
            if counts[hit.article_id] >= 2:
                continue
            counts[hit.article_id] += 1
            result.append(hit)
            if len(result) == self.top_k:
                break
        return result

    async def aclose(self):
        await self.store.aclose()

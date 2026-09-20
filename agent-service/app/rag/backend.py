import asyncio
from typing import Protocol

from qdrant_client import models

from app.clients.models import ArticleVersionChunks
from app.rag.embedding import EmbeddingProvider, validate_vectors
from app.rag.errors import RagConfigurationError, RagError
from app.rag.models import RetrievedChunk
from app.rag.store import QdrantChunkStore, chunk_payload, point_id, version_filter
from app.rag.verified_write import upsert_verify_delete


class RetrievalBackend(Protocol):
    async def replace(self, source: ArticleVersionChunks) -> None: ...

    async def search(self, source: ArticleVersionChunks, query: str, limit: int) -> list[RetrievedChunk]: ...

    async def aclose(self) -> None: ...


class LocalRetrievalBackend:
    def __init__(self, embedding: EmbeddingProvider, store: QdrantChunkStore):
        if embedding.dimension != store.dimension or embedding.profile != store.profile:
            raise RagConfigurationError("Embedding provider and vector store profiles differ")
        self.embedding = embedding
        self.store = store

    async def replace(self, source: ArticleVersionChunks) -> None:
        # 本地全部推理和向量校验必须发生在任何旧点删除之前。
        try:
            vectors = await asyncio.to_thread(self.embedding.embed_documents, [c.text for c in source.chunks])
            validate_vectors(vectors, len(source.chunks), self.embedding.dimension)
        except RagError:
            raise
        except Exception:
            raise RagError("Embedding failed; existing index was not replaced") from None
        await self.store.replace(source, vectors)

    async def search(self, source: ArticleVersionChunks, query: str, limit: int) -> list[RetrievedChunk]:
        try:
            vector = await asyncio.to_thread(self.embedding.embed_query, query)
        except RagError:
            raise
        except Exception:
            raise RagError("Query embedding failed") from None
        return await self.store.search(source.user_id, source.article_id, source.version_no, vector, limit)

    async def aclose(self) -> None:
        await self.store.aclose()


class QdrantCloudRetrievalBackend:
    # SDK 请求分批，所有 upsert 和全部验证完成后，才允许执行第一批 stale delete。
    _batch_size = 64

    def __init__(self, store: QdrantChunkStore):
        if store.profile["provider"] != "qdrant_cloud":
            raise RagConfigurationError("Cloud inference requires the qdrant_cloud profile")
        self.store = store
        self.model = str(store.profile["model"])
        self._write_lock = asyncio.Lock()

    async def _existing_ids(self, source: ArticleVersionChunks) -> set[int | str]:
        existing = set()
        offset = None
        seen_offsets = set()
        while True:
            records, next_offset = await self.store.client.scroll(
                self.store.collection,
                scroll_filter=version_filter(source.user_id, source.article_id, source.version_no),
                limit=self._batch_size, offset=offset,
                with_payload=["user_id", "article_id", "version_no"], with_vectors=False,
            )
            for record in records:
                payload = record.payload or {}
                if (payload.get("user_id"), payload.get("article_id"), payload.get("version_no")) != (source.user_id, source.article_id, source.version_no):
                    raise RagError("Existing index returned a mismatched scope")
                existing.add(record.id)
            if next_offset is None:
                return existing
            if next_offset in seen_offsets:
                raise RagError("Existing index pagination did not advance")
            seen_offsets.add(next_offset)
            offset = next_offset

    async def replace(self, source: ArticleVersionChunks) -> None:
        try:
            async with self._write_lock:
                await self.store.ensure_collection(create=True)
                existing_ids = await self._existing_ids(source)
                points = [models.PointStruct(
                    id=point_id(source.article_id, source.version_no, chunk.index),
                    # Cloud E5 在服务端区分 passage/query；这里必须传 Go 原文。
                    vector=models.Document(text=chunk.text, model=self.model),
                    payload=chunk_payload(source, chunk),
                ) for chunk in source.chunks]
                await upsert_verify_delete(self.store.client, self.store.collection, points, existing_ids, self._batch_size)
        except RagError:
            raise
        except Exception:
            raise RagError("Cloud version index replacement failed; explicit retry is required") from None

    async def search(self, source: ArticleVersionChunks, query: str, limit: int) -> list[RetrievedChunk]:
        return await self.store.search_document(
            source.user_id, source.article_id, source.version_no,
            models.Document(text=query, model=self.model), limit,
        )

    async def aclose(self) -> None:
        await self.store.aclose()

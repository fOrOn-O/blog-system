import asyncio
from uuid import NAMESPACE_URL, uuid5

from qdrant_client import AsyncQdrantClient, models

from app.clients.models import ArticleVersionChunks, ArticleChunk
from app.core.config import Settings
from app.rag.embedding import validate_vectors
from app.rag.errors import RagConfigurationError, RagError
from app.rag.models import RetrievedChunk


def point_id(article_id: int, version_no: int, chunk_index: int) -> str:
    return str(uuid5(NAMESPACE_URL, f"blog-system/article/{article_id}/version/{version_no}/chunk/{chunk_index}"))


def version_filter(user_id: int, article_id: int, version_no: int) -> models.Filter:
    return models.Filter(must=[
        models.FieldCondition(key=key, match=models.MatchValue(value=value))
        for key, value in (("user_id", user_id), ("article_id", article_id), ("version_no", version_no))
    ])


def chunk_payload(source: ArticleVersionChunks, chunk: ArticleChunk) -> dict:
    return {
        "user_id": source.user_id, "article_id": source.article_id, "version_no": source.version_no,
        "chunk_index": chunk.index,
        "heading_path": [h.model_dump(mode="json") for h in chunk.heading_path], "text": chunk.text,
    }


class QdrantChunkStore:
    def __init__(self, client: AsyncQdrantClient, settings: Settings):
        self.client = client
        self.collection = settings.qdrant_collection
        self.dimension = settings.embedding_dimension
        self.distance = models.Distance(settings.qdrant_distance)
        self.profile = settings.embedding_profile
        self._setup_lock = asyncio.Lock()
        self._write_lock = asyncio.Lock()

    async def ensure_collection(self, *, create: bool) -> bool:
        # 查询也校验实际集合规格；绝不静默重建或删除不匹配的集合。
        async with self._setup_lock:
            if not await self.client.collection_exists(self.collection):
                if not create:
                    return False
                await self.client.create_collection(
                    self.collection, vectors_config=models.VectorParams(size=self.dimension, distance=self.distance),
                    metadata={"embedding_profile": self.profile},
                )
            info = await self.client.get_collection(self.collection)
            vectors = info.config.params.vectors
            if not isinstance(vectors, models.VectorParams) or vectors.size != self.dimension or vectors.distance != self.distance:
                raise RagConfigurationError("Qdrant collection vector size/distance does not match embedding configuration")
            if (info.config.metadata or {}).get("embedding_profile") != self.profile:
                raise RagConfigurationError("Qdrant collection embedding profile is missing or incompatible; use a new collection")
            if create:
                for name in ("user_id", "article_id", "version_no"):
                    existing = info.payload_schema.get(name)
                    if existing is not None and existing.data_type != models.PayloadSchemaType.INTEGER:
                        raise RagConfigurationError("Qdrant payload index type does not match configuration")
                    if existing is None:
                        await self.client.create_payload_index(
                            self.collection, field_name=name, field_schema=models.PayloadSchemaType.INTEGER, wait=True,
                        )
            return True

    async def replace(self, source: ArticleVersionChunks, vectors: list[list[float]]) -> None:
        # 校验和 Point 构造都发生在删除之前；空分块结果也需要清除旧点。
        validate_vectors(vectors, len(source.chunks), self.dimension)
        points = [models.PointStruct(
            id=point_id(source.article_id, source.version_no, chunk.index), vector=vector,
            payload=chunk_payload(source, chunk),
        ) for chunk, vector in zip(source.chunks, vectors, strict=True)]
        try:
            async with self._write_lock:
                await self.ensure_collection(create=True)
                await self.client.delete(
                    self.collection,
                    points_selector=models.FilterSelector(filter=version_filter(source.user_id, source.article_id, source.version_no)),
                    wait=True,
                )
                if points:
                    await self.client.upsert(self.collection, points=points, wait=True)
        except RagError:
            raise
        except Exception:
            raise RagError("Version index replacement failed; explicit retry is required") from None

    async def search(self, user_id: int, article_id: int, version_no: int, vector: list[float], top_k: int) -> list[RetrievedChunk]:
        validate_vectors([vector], 1, self.dimension)
        return await self._search(user_id, article_id, version_no, vector, top_k)

    async def search_document(self, user_id: int, article_id: int, version_no: int, document: models.Document, top_k: int) -> list[RetrievedChunk]:
        return await self._search(user_id, article_id, version_no, document, top_k)

    async def _search(self, user_id: int, article_id: int, version_no: int, query: list[float] | models.Document, top_k: int) -> list[RetrievedChunk]:
        try:
            if not await self.ensure_collection(create=False):
                return []
            result = await self.client.query_points(
                self.collection, query=query, query_filter=version_filter(user_id, article_id, version_no),
                limit=top_k, with_payload=True, with_vectors=False,
            )
            chunks = []
            for point in result.points:
                payload = point.payload or {}
                # 除过滤器外再次检查身份，不将错误或污染的跨域 payload 传入 Agent。
                if (payload.get("user_id"), payload.get("article_id"), payload.get("version_no")) != (user_id, article_id, version_no):
                    raise RagError("Retrieval returned a mismatched scope")
                chunk = RetrievedChunk.model_validate({**payload, "score": point.score})
                if str(point.id) != point_id(article_id, version_no, chunk.chunk_index):
                    raise RagError("Retrieval returned an unexpected point identity")
                chunks.append(chunk)
            return chunks
        except RagError:
            raise
        except Exception:
            raise RagError("Version retrieval failed") from None

    async def aclose(self) -> None:
        await self.client.close()

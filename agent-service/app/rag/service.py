import asyncio

from app.clients.blog import BlogClient
from app.rag.embedding import EmbeddingProvider, validate_vectors
from app.rag.errors import RagConfigurationError, RagError
from app.rag.models import RetrievedChunk
from app.rag.store import QdrantChunkStore


class ArticleRagService:
    def __init__(self, embedding: EmbeddingProvider, store: QdrantChunkStore, *, top_k: int):
        if embedding.dimension != store.dimension or embedding.profile != store.profile:
            raise RagConfigurationError("Embedding provider and vector store profiles differ")
        self.embedding = embedding
        self.store = store
        self.top_k = top_k

    async def index_article_version(self, article_id: int, version_no: int, *, access_token: str, blog_client: BlogClient) -> int:
        source = await blog_client.get_version_chunks(article_id, version_no, access_token=access_token)
        # CPU 推理在线程执行；全部成功并验证后才允许替换索引。
        try:
            vectors = await asyncio.to_thread(self.embedding.embed_documents, [c.text for c in source.chunks])
            validate_vectors(vectors, len(source.chunks), self.embedding.dimension)
        except RagError:
            raise
        except Exception:
            raise RagError("Embedding failed; existing index was not replaced") from None
        await self.store.replace(source, vectors)
        return len(source.chunks)

    async def search_article_version(self, article_id: int, version_no: int, query: str, *, access_token: str, blog_client: BlogClient) -> list[RetrievedChunk]:
        if not query.strip():
            raise ValueError("query must not be empty")
        # 每次检索都由 Go 重新确认文章、版本存在及所有权；不信任客户端传入 user_id。
        source = await blog_client.get_version_chunks(article_id, version_no, access_token=access_token)
        if not source.chunks:
            return []
        try:
            vector = await asyncio.to_thread(self.embedding.embed_query, query)
        except RagError:
            raise
        except Exception:
            raise RagError("Query embedding failed") from None
        found = await self.store.search(source.user_id, source.article_id, source.version_no, vector, self.top_k)
        canonical = {c.index: c for c in source.chunks}
        result = []
        seen = set()
        for hit in found:
            current = canonical.get(hit.chunk_index)
            # Qdrant 仅决定检索排序；内容必须仍与 Go 一致，失效索引不能成为事实来源。
            if current is None or current.text != hit.text or current.heading_path != hit.heading_path or hit.chunk_index in seen:
                continue
            seen.add(hit.chunk_index)
            result.append(hit.model_copy(update={"text": current.text, "heading_path": current.heading_path}))
        return result

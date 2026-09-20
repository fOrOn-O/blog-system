from app.clients.blog import BlogClient
from app.rag.backend import RetrievalBackend
from app.rag.errors import RagError
from app.rag.models import RetrievedChunk


class ArticleRagService:
    def __init__(self, backend: RetrievalBackend, *, top_k: int):
        self.backend = backend
        self.top_k = top_k

    async def aclose(self) -> None:
        await self.backend.aclose()

    async def index_article_version(self, article_id: int, version_no: int, *, access_token: str, blog_client: BlogClient) -> int:
        source = await blog_client.get_version_chunks(article_id, version_no, access_token=access_token)
        await self.backend.replace(source)
        return len(source.chunks)

    async def search_article_version(self, article_id: int, version_no: int, query: str, *, access_token: str, blog_client: BlogClient) -> list[RetrievedChunk]:
        if not query.strip():
            raise ValueError("query must not be empty")
        # 每次检索都由 Go 重新确认文章、版本存在及所有权；不信任客户端传入 user_id。
        source = await blog_client.get_version_chunks(article_id, version_no, access_token=access_token)
        if not source.chunks:
            return []
        candidate_k = min(self.top_k * 3, 100)
        found = await self.backend.search(source, query, candidate_k)
        canonical = {c.index: c for c in source.chunks}
        result = []
        seen = set()
        for hit in found:
            if hit.article_id != source.article_id or hit.version_no != source.version_no:
                raise RagError("Retrieval returned a mismatched scope")
            current = canonical.get(hit.chunk_index)
            # Qdrant 仅决定检索排序；内容必须仍与 Go 一致，失效索引不能成为事实来源。
            if current is None or current.text != hit.text or current.heading_path != hit.heading_path or hit.chunk_index in seen:
                continue
            seen.add(hit.chunk_index)
            result.append(hit.model_copy(update={"text": current.text, "heading_path": current.heading_path}))
            if len(result) == self.top_k:
                break
        return result

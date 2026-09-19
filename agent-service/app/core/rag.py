from functools import lru_cache

from qdrant_client import AsyncQdrantClient

from app.core.config import Settings, get_settings
from app.rag.embedding import EmbeddingProvider, LocalE5EmbeddingProvider
from app.rag.errors import RagConfigurationError
from app.rag.service import ArticleRagService
from app.rag.store import QdrantChunkStore


def create_embedding_provider(settings: Settings) -> EmbeddingProvider:
    # 新增远程 Provider 时仅在这里注册；索引、检索和工具不依赖具体实现。
    if settings.embedding_provider == "local_e5":
        if settings.app_env.lower() in {"production", "prod"}:
            raise RagConfigurationError("Production requires an explicitly implemented remote embedding provider")
        return LocalE5EmbeddingProvider(settings)
    raise RagConfigurationError("Configured embedding provider is not implemented")


@lru_cache
def get_embedding_provider() -> EmbeddingProvider:
    return create_embedding_provider(get_settings())


@lru_cache
def get_rag_service() -> ArticleRagService:
    settings = get_settings()
    embedding = get_embedding_provider()
    client = AsyncQdrantClient(
        url=str(settings.qdrant_url), timeout=settings.qdrant_timeout_seconds,
        api_key=settings.qdrant_api_key.get_secret_value() or None,
        check_compatibility=False,
    )
    return ArticleRagService(
        embedding, QdrantChunkStore(client, settings), top_k=settings.rag_top_k,
    )


async def close_rag_service() -> None:
    if get_rag_service.cache_info().currsize:
        await get_rag_service().store.aclose()
        get_rag_service.cache_clear()

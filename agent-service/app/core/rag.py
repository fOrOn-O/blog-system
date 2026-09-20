from functools import lru_cache

from qdrant_client import AsyncQdrantClient

from app.core.config import Settings, get_settings
from app.rag.embedding import EmbeddingProvider, LocalE5EmbeddingProvider
from app.rag.backend import LocalRetrievalBackend, QdrantCloudRetrievalBackend
from app.rag.errors import RagConfigurationError
from app.rag.service import ArticleRagService
from app.rag.store import QdrantChunkStore
from app.knowledge.service import PublishedKnowledgeService


def create_embedding_provider(settings: Settings) -> EmbeddingProvider:
    # 此工厂只负责本地 float 向量；Cloud Inference 由 RetrievalBackend 处理。
    if settings.embedding_provider == "local_e5":
        if settings.app_env.lower() in {"production", "prod"}:
            raise RagConfigurationError("Production requires an explicitly implemented remote embedding provider")
        return LocalE5EmbeddingProvider(settings)
    if settings.embedding_provider == "qdrant_cloud":
        raise RagConfigurationError("warm-model only supports local_e5; qdrant_cloud has no local embedding model")
    raise RagConfigurationError("Configured embedding provider is not implemented")


@lru_cache
def get_embedding_provider() -> EmbeddingProvider:
    return create_embedding_provider(get_settings())


@lru_cache
def get_rag_service() -> ArticleRagService:
    settings = get_settings()
    if settings.embedding_provider == "local_e5":
        embedding = get_embedding_provider()
    elif settings.embedding_provider != "qdrant_cloud":
        raise RagConfigurationError("Configured embedding provider is not implemented")
    client = AsyncQdrantClient(
        url=str(settings.qdrant_url), timeout=settings.qdrant_timeout_seconds,
        api_key=settings.qdrant_api_key.get_secret_value() or None,
        check_compatibility=False,
        cloud_inference=settings.embedding_provider == "qdrant_cloud",
    )
    store = QdrantChunkStore(client, settings)
    backend = (LocalRetrievalBackend(embedding, store) if settings.embedding_provider == "local_e5"
               else QdrantCloudRetrievalBackend(store))
    return ArticleRagService(backend, top_k=settings.rag_top_k)


async def close_rag_service() -> None:
    if get_rag_service.cache_info().currsize:
        await get_rag_service().aclose()
        get_rag_service.cache_clear()


@lru_cache
def get_published_service() -> PublishedKnowledgeService:
    settings = get_settings()
    embedding = None
    if settings.embedding_provider == "local_e5":
        embedding = get_embedding_provider()
    elif settings.embedding_provider != "qdrant_cloud":
        raise RagConfigurationError("Configured embedding provider is not implemented")
    client = AsyncQdrantClient(url=str(settings.qdrant_url), timeout=settings.qdrant_timeout_seconds,
        api_key=settings.qdrant_api_key.get_secret_value() or None, check_compatibility=False,
        cloud_inference=settings.embedding_provider == "qdrant_cloud")
    # 独立集合及 metadata；同一进程复用本地模型，不重复下载或加载。
    profile = settings.model_copy(update={"qdrant_collection": settings.published_knowledge_collection})
    return PublishedKnowledgeService(QdrantChunkStore(client, profile, index_fields=("article_id", "version_no")),
                                     embedding=embedding, top_k=settings.rag_top_k)


async def close_published_service() -> None:
    if get_published_service.cache_info().currsize:
        await get_published_service().aclose()
        get_published_service.cache_clear()

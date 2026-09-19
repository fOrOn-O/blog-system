import math
from threading import Lock
from typing import Protocol

from app.core.config import Settings
from app.rag.errors import EmbeddingError, RagConfigurationError


class EmbeddingProvider(Protocol):
    dimension: int
    profile: dict[str, str | int]

    def embed_documents(self, texts: list[str]) -> list[list[float]]: ...

    def embed_query(self, text: str) -> list[float]: ...


def validate_vectors(vectors: list[list[float]], count: int, dimension: int) -> None:
    if len(vectors) != count:
        raise EmbeddingError("Embedding count does not match input")
    for vector in vectors:
        if len(vector) != dimension or not all(math.isfinite(v) for v in vector):
            raise EmbeddingError("Embedding vector has invalid dimension or values")
        norm_squared = sum(v * v for v in vector)
        if not math.isfinite(norm_squared) or norm_squared <= 0:
            raise EmbeddingError("Embedding vector has invalid norm")


class LocalE5EmbeddingProvider:
    """每个应用实例只加载一次模型；锁同时保护首次加载和本地推理。"""

    def __init__(self, settings: Settings):
        self.dimension = settings.embedding_dimension
        self.profile = settings.embedding_profile
        self._name = settings.embedding_model
        self._device = settings.embedding_device
        self._batch_size = settings.embedding_batch_size
        self._model = None
        self._lock = Lock()

    def _load_model(self):
        # 延迟导入和加载，健康检查与使用 Fake Provider 的测试不下载模型。
        from sentence_transformers import SentenceTransformer

        return SentenceTransformer(self._name, device=self._device, trust_remote_code=False)

    def _get_model(self):
        if self._model is None:
            model = self._load_model()
            if model.get_sentence_embedding_dimension() != self.dimension:
                raise RagConfigurationError("Embedding model dimension does not match configuration")
            self._model = model
        return self._model

    def warm(self) -> None:
        self.embed_query("warm up")

    def _encode(self, texts: list[str]) -> list[list[float]]:
        if not texts:
            return []
        try:
            with self._lock:
                values = self._get_model().encode(
                    texts, batch_size=self._batch_size, normalize_embeddings=True,
                    convert_to_numpy=True, show_progress_bar=False,
                ).tolist()
            validate_vectors(values, len(texts), self.dimension)
            return values
        except (EmbeddingError, RagConfigurationError):
            raise
        except Exception:
            raise EmbeddingError("Local embedding inference failed") from None

    def embed_documents(self, texts: list[str]) -> list[list[float]]:
        return self._encode(["passage: " + text for text in texts])

    def embed_query(self, text: str) -> list[float]:
        if not text.strip():
            raise ValueError("query must not be empty")
        return self._encode(["query: " + text])[0]

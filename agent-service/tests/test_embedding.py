from concurrent.futures import ThreadPoolExecutor
from unittest.mock import Mock

import pytest
from pydantic import ValidationError

from app.core.config import Settings
from app.core.rag import create_embedding_provider
from app.rag.embedding import LocalE5EmbeddingProvider, validate_vectors
from app.rag.errors import EmbeddingError, RagConfigurationError


def test_e5_prefix_order_normalization_and_single_load(monkeypatch):
    settings = Settings(_env_file=None)
    provider = LocalE5EmbeddingProvider(settings)
    encoder = Mock()
    encoder.get_sentence_embedding_dimension.return_value = 384
    inputs = []

    def encode(texts, **kwargs):
        inputs.append((texts, kwargs))
        return Mock(tolist=lambda: [[float(i + 1)] + [0.] * 383 for i in range(len(texts))])

    encoder.encode.side_effect = encode
    load = Mock(return_value=encoder)
    monkeypatch.setattr(provider, "_load_model", load)
    texts = ["首个块", "第二个块"]
    assert provider.embed_documents(texts) == [[1.] + [0.] * 383, [2.] + [0.] * 383]
    with ThreadPoolExecutor(max_workers=4) as pool:
        assert len(list(pool.map(provider.embed_query, ["问题"] * 4))) == 4
    load.assert_called_once()
    assert inputs[0][0] == ["passage: 首个块", "passage: 第二个块"]
    assert all(row[0] == ["query: 问题"] for row in inputs[1:])
    assert all(row[1]["normalize_embeddings"] and not row[1]["show_progress_bar"] for row in inputs)
    assert inputs[0][1]["batch_size"] == settings.embedding_batch_size
    assert texts == ["首个块", "第二个块"]


def test_empty_documents_do_not_load_model_and_wrong_model_dimension_fails(monkeypatch):
    provider = LocalE5EmbeddingProvider(Settings(_env_file=None))
    model = Mock()
    model.get_sentence_embedding_dimension.return_value = 768
    load = Mock(return_value=model)
    monkeypatch.setattr(provider, "_load_model", load)
    assert provider.embed_documents([]) == []
    load.assert_not_called()
    with pytest.raises(ValueError):
        provider.embed_query(" ")
    with pytest.raises(RagConfigurationError):
        provider.embed_documents(["text"])
    model.encode.assert_not_called()


@pytest.mark.parametrize("vectors,count", [([], 1), ([[1.]], 1), ([[0.] * 384], 1), ([[float("nan")] * 384], 1)])
def test_vector_validation(vectors, count):
    with pytest.raises(EmbeddingError):
        validate_vectors(vectors, count, 384)


def test_embedding_errors_do_not_expose_backend_details(monkeypatch):
    provider = LocalE5EmbeddingProvider(Settings(_env_file=None))
    monkeypatch.setattr(provider, "_load_model", Mock(side_effect=RuntimeError("private-error-details")))
    with pytest.raises(EmbeddingError) as error:
        provider.embed_query("question")
    assert "private-error-details" not in str(error.value)


@pytest.mark.parametrize("values", [
    {"embedding_dimension": 768}, {"embedding_model": "different-model"}, {"qdrant_distance": "Dot"},
    {"rag_top_k": 0}, {"qdrant_api_key": "fake-key", "qdrant_url": "http://vector.test"},
])
def test_incoherent_or_unsafe_configuration_is_rejected(values):
    with pytest.raises(ValidationError):
        Settings(_env_file=None, **values)


def test_environment_profile_overrides_dotenv_without_local_fallback(tmp_path, monkeypatch):
    env = tmp_path / ".env"
    env.write_text("EMBEDDING_PROVIDER=local_e5\nQDRANT_COLLECTION=article_chunks_e5_v1\n", encoding="utf-8")
    for key, value in {
        "APP_ENV": "production", "EMBEDDING_PROVIDER": "remote_example",
        "EMBEDDING_MODEL": "remote-model-v1", "EMBEDDING_DIMENSION": "768",
        "QDRANT_DISTANCE": "Dot", "QDRANT_COLLECTION": "article_chunks_remote_v1",
        "QDRANT_URL": "https://vector.example.test:6333", "QDRANT_API_KEY": "fake-api-key",
    }.items():
        monkeypatch.setenv(key, value)
    settings = Settings(_env_file=env)
    assert settings.embedding_profile == {
        "provider": "remote_example", "model": "remote-model-v1", "dimension": 768,
        "distance": "Dot", "collection": "article_chunks_remote_v1",
    }
    assert "fake-api-key" not in repr(settings)
    with pytest.raises(RagConfigurationError, match="not implemented"):
        create_embedding_provider(settings)


def test_production_cannot_accidentally_load_local_model():
    with pytest.raises(RagConfigurationError, match="Production"):
        create_embedding_provider(Settings(_env_file=None, app_env="production"))


def test_profile_validation_error_does_not_print_secrets():
    with pytest.raises(ValidationError) as error:
        Settings(_env_file=None, qdrant_api_key="fake-secret-must-not-leak", embedding_dimension=768)
    assert "fake-secret-must-not-leak" not in str(error.value)


@pytest.mark.anyio
async def test_centralized_clients_cached_and_closed(monkeypatch):
    from unittest.mock import AsyncMock
    from app.core import rag

    settings = Settings(_env_file=None, qdrant_url="https://vector.example.test", qdrant_api_key="fake-key")
    client = Mock(close=AsyncMock())
    constructor = Mock(return_value=client)
    monkeypatch.setattr(rag, "get_settings", lambda: settings)
    monkeypatch.setattr(rag, "AsyncQdrantClient", constructor)
    rag.get_embedding_provider.cache_clear()
    rag.get_rag_service.cache_clear()
    try:
        first = rag.get_rag_service()
        assert first is rag.get_rag_service()
        assert first.backend.embedding is rag.get_embedding_provider()
        assert first.backend.embedding._model is None
        constructor.assert_called_once_with(url="https://vector.example.test/", timeout=10, api_key="fake-key", check_compatibility=False, cloud_inference=False)
        await rag.close_rag_service()
        client.close.assert_awaited_once()
        assert rag.get_rag_service.cache_info().currsize == 0
    finally:
        rag.get_embedding_provider.cache_clear()
        rag.get_rag_service.cache_clear()


@pytest.fixture
def anyio_backend():
    return "asyncio"

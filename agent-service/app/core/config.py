from functools import lru_cache
from typing import Literal

from pydantic import Field, HttpUrl, SecretStr, field_validator, model_validator
from pydantic_settings import BaseSettings, SettingsConfigDict


class Settings(BaseSettings):
    model_config = SettingsConfigDict(env_file=".env", env_file_encoding="utf-8", hide_input_in_errors=True)

    app_name: str = "blog-agent"
    app_env: str = "development"
    log_level: Literal["DEBUG", "INFO", "WARNING", "ERROR", "CRITICAL"] = "INFO"
    blog_backend_url: HttpUrl = HttpUrl("http://localhost:8080")
    blog_backend_timeout_seconds: float = Field(default=10, gt=0, allow_inf_nan=False)
    groq_api_key: SecretStr = Field(default=SecretStr(""), repr=False)
    llm_model: str = Field(default="openai/gpt-oss-20b", min_length=1)
    llm_timeout_seconds: float = Field(default=30, gt=0, allow_inf_nan=False)
    agent_recursion_limit: int = Field(default=12, ge=2, le=50)
    qdrant_url: HttpUrl = HttpUrl("http://localhost:6333")
    qdrant_api_key: SecretStr = Field(default=SecretStr(""), repr=False)
    qdrant_collection: str = Field(default="article_chunks_e5_v1", pattern=r"^[A-Za-z0-9_-]+$")
    qdrant_timeout_seconds: float = Field(default=10, gt=0, allow_inf_nan=False)
    embedding_provider: str = Field(default="local_e5", min_length=1)
    embedding_model: str = Field(default="intfloat/multilingual-e5-small", min_length=1)
    embedding_dimension: int = Field(default=384, ge=1)
    qdrant_distance: Literal["Cosine", "Dot", "Euclid", "Manhattan"] = "Cosine"
    embedding_device: str = Field(default="cpu", min_length=1)
    embedding_batch_size: int = Field(default=32, ge=1, le=256)
    rag_top_k: int = Field(default=5, ge=1, le=50)

    @model_validator(mode="after")
    def validate_embedding_profile(self):
        if self.embedding_provider in {"local_e5", "qdrant_cloud"} and (
            self.embedding_model != "intfloat/multilingual-e5-small"
            or self.embedding_dimension != 384 or self.qdrant_distance != "Cosine"
        ):
            raise ValueError("E5 profiles require intfloat/multilingual-e5-small, 384 dimensions and Cosine")
        if self.embedding_provider == "qdrant_cloud" and (
            self.qdrant_url.scheme != "https" or not self.qdrant_api_key.get_secret_value().strip()
        ):
            raise ValueError("qdrant_cloud requires an HTTPS QDRANT_URL and nonempty QDRANT_API_KEY")
        if self.qdrant_api_key.get_secret_value() and self.qdrant_url.scheme != "https":
            raise ValueError("Authenticated Qdrant requires HTTPS")
        return self

    @property
    def embedding_profile(self) -> dict[str, str | int]:
        return {
            "provider": self.embedding_provider, "model": self.embedding_model,
            "dimension": self.embedding_dimension, "distance": self.qdrant_distance,
            "collection": self.qdrant_collection,
        }

    @field_validator("blog_backend_url", "qdrant_url")
    @classmethod
    def validate_backend_origin(cls, value: HttpUrl) -> HttpUrl:
        if value.username or value.password or value.query or value.fragment:
            raise ValueError("Backend URL must not include credentials, query or fragment")
        if value.path not in (None, "/"):
            raise ValueError("Backend URL must be an origin without an API path")
        return value


@lru_cache
def get_settings() -> Settings:
    return Settings()

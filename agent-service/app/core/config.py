from functools import lru_cache
from typing import Literal

from pydantic import Field, HttpUrl, field_validator
from pydantic_settings import BaseSettings, SettingsConfigDict


class Settings(BaseSettings):
    model_config = SettingsConfigDict(env_file=".env", env_file_encoding="utf-8")

    app_name: str = "blog-agent"
    app_env: str = "development"
    log_level: Literal["DEBUG", "INFO", "WARNING", "ERROR", "CRITICAL"] = "INFO"
    blog_backend_url: HttpUrl = HttpUrl("http://localhost:8080")
    blog_backend_timeout_seconds: float = Field(default=10, gt=0, allow_inf_nan=False)

    @field_validator("blog_backend_url")
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

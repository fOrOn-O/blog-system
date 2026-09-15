from langchain_groq import ChatGroq

from app.core.config import Settings, get_settings


class ModelConfigurationError(ValueError):
    """A model cannot be created from the current non-secret configuration."""


def create_model(settings: Settings | None = None) -> ChatGroq:
    settings = settings if settings is not None else get_settings()
    if not settings.groq_api_key.get_secret_value().strip():
        raise ModelConfigurationError("GROQ_API_KEY is required to run the real model")
    return ChatGroq(
        model=settings.llm_model,
        api_key=settings.groq_api_key,
        timeout=settings.llm_timeout_seconds,
        # Groq counts retries after the initial model API request; 0 disables them.
        # Tool execution and its retry behavior are independent of this setting.
        max_retries=0,
    )

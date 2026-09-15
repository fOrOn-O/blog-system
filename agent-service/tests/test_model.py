import asyncio

import groq
import httpx
import pytest
from langchain_groq import ChatGroq
from pydantic import SecretStr

from app.core.config import Settings
from app.core.model import ModelConfigurationError, create_model


def test_missing_key_is_only_required_when_creating_real_model(monkeypatch):
    def forbidden(**kwargs):
        pytest.fail("Missing credentials must be rejected before provider initialization")

    monkeypatch.setattr("app.core.model.ChatGroq", forbidden)
    settings = Settings(_env_file=None, groq_api_key="")
    with pytest.raises(ModelConfigurationError, match="GROQ_API_KEY"):
        create_model(settings)


def test_factory_uses_single_provider_and_disables_sdk_retries(monkeypatch):
    captured = {}
    model = object()

    def factory(**kwargs):
        captured.update(kwargs)
        return model

    monkeypatch.setattr("app.core.model.ChatGroq", factory)
    settings = Settings(
        _env_file=None, groq_api_key="test-provider-key", llm_model="openai/gpt-oss-20b",
        llm_timeout_seconds=15,
    )
    assert create_model(settings) is model
    assert captured == {
        "model": "openai/gpt-oss-20b", "api_key": SecretStr("test-provider-key"),
        "timeout": 15, "max_retries": 0,
    }
    assert "test-provider-key" not in repr(settings)
    assert "test-provider-key" not in settings.model_dump_json()


@pytest.mark.parametrize(
    ("failure", "expected_error"),
    [(500, groq.InternalServerError), (429, groq.RateLimitError),
     ("connection", groq.APIConnectionError)],
)
def test_real_sdk_does_not_retry_model_requests(monkeypatch, failure, expected_error):
    monkeypatch.setenv("LANGSMITH_TRACING", "false")
    monkeypatch.setenv("LANGCHAIN_TRACING_V2", "false")
    requests = []

    def fail_request(request):
        requests.append(request)
        if failure == "connection":
            raise httpx.ConnectError("Simulated connection failure", request=request)
        return httpx.Response(failure, json={"error": {"message": "Simulated failure"}})

    async def run():
        transport = httpx.MockTransport(fail_request)
        with httpx.Client(transport=transport) as sync_client:
            async with httpx.AsyncClient(transport=transport) as async_client:
                def factory(**kwargs):
                    return ChatGroq(
                        **kwargs, http_client=sync_client, http_async_client=async_client,
                    )

                monkeypatch.setattr("app.core.model.ChatGroq", factory)
                model = create_model(Settings(
                    _env_file=None, groq_api_key="test-provider-key",
                    llm_model="openai/gpt-oss-20b",
                ))
                with pytest.raises(expected_error):
                    await model.ainvoke("Test retry behavior")
        assert len(requests) == 1

    asyncio.run(run())

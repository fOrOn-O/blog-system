from unittest.mock import AsyncMock, Mock

import httpx
import pytest
from fastapi import FastAPI
from fastapi.testclient import TestClient

from app.api import chat
from app.agent.models import AgentResponse
from app.clients.blog import BlogClient
from app.core.config import Settings
from test_article_version_client import version_data


@pytest.fixture
def setup_chat(monkeypatch):
    settings = Settings(_env_file=None, groq_api_key="")
    monkeypatch.setattr(chat, "get_settings", lambda: settings)
    calls = []
    status = [200]

    def transport(request):
        calls.append(request)
        return httpx.Response(status[0], json={"code": status[0], "data": version_data(), "message": "private-detail"})

    monkeypatch.setattr(chat, "BlogClient", lambda _: BlogClient(settings, transport=httpx.MockTransport(transport)))
    runner = Mock()
    runner.run_with_response = AsyncMock(return_value=AgentResponse(answer="你好", proposal=None))
    factory = Mock(return_value=runner)
    app = FastAPI()
    app.include_router(chat.router)
    app.dependency_overrides[chat.runner_factory] = lambda: factory
    return TestClient(app), runner, factory, calls, status


def body():
    return {"message": "你好", "workspace": {"article_id": 23, "version_no": 7}}


def test_chat_verifies_workspace_and_injects_jwt(setup_chat):
    client, runner, factory, calls, _ = setup_chat
    response = client.post("/api/v1/agent/chat", json=body(), headers={"Authorization": "Bearer test-token"})
    assert response.status_code == 200 and response.json() == {"answer": "你好", "proposal": None}
    assert len(calls) == 1 and calls[0].url.path == "/api/v1/agent/articles/23/versions/7"
    assert calls[0].headers["Authorization"] == "Bearer test-token"
    kwargs = runner.run_with_response.call_args.kwargs
    assert kwargs["access_token"] == "test-token"
    assert kwargs["workspace"].model_dump() == body()["workspace"]
    assert "test-token" not in response.text


@pytest.mark.parametrize("status", [401, 403, 404, 500])
def test_chat_rejects_go_auth_errors_before_model(setup_chat, status):
    client, runner, factory, _, response_status = setup_chat
    response_status[0] = status
    response = client.post("/api/v1/agent/chat", json=body(), headers={"Authorization": "Bearer test-token"})
    assert response.status_code == (502 if status == 500 else status)
    factory.assert_not_called()
    assert "private-detail" not in response.text


def test_chat_requires_auth_and_strict_workspace(setup_chat):
    client, _, factory, calls, _ = setup_chat
    assert client.post("/api/v1/agent/chat", json=body()).status_code == 401
    for invalid in [{**body(), "user_id": 8}, {**body(), "workspace": {"article_id": 23, "version_no": 7, "access_token": "forged"}}]:
        assert client.post("/api/v1/agent/chat", json=invalid, headers={"Authorization": "Bearer test-token"}).status_code == 422
    factory.assert_not_called()
    assert not calls

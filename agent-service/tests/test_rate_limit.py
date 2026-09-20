from concurrent.futures import ThreadPoolExecutor
from unittest.mock import AsyncMock, Mock

import httpx
from fastapi import FastAPI
from fastapi.testclient import TestClient

from app.api import chat, knowledge
from app.api.health import router as health_router
from app.agent.models import AgentResponse
from app.clients.blog import BlogClient
from app.core.config import Settings
from app.core import rate_limit
from test_article_version_client import version_data


def test_sliding_window_subject_isolation_expiry_capacity_and_concurrency():
    clock = [0.]
    limiter = rate_limit.RequestLimiter(10, 60, clock=lambda: clock[0], max_subjects=2)
    with ThreadPoolExecutor(max_workers=12) as pool:
        results = list(pool.map(lambda _: limiter.retry_after(("user", 7)), range(30)))
    assert results.count(0) == 10 and results.count(60) == 20
    assert limiter.retry_after(("user", 8)) == 0
    assert limiter.retry_after(("user", 9)) == 60
    clock[0] = 59.1
    assert limiter.retry_after(("user", 7)) == 1
    clock[0] = 60
    assert limiter.retry_after(("user", 9)) == 0
    assert set(limiter._buckets) == {("user", 9)}
    # 命名空间保留未来可信 IP adapter 的扩展点。
    assert limiter.retry_after(("ip", "127.0.0.1")) == 0


def test_chat_and_knowledge_share_verified_user_limit_before_model_and_retrieval(monkeypatch):
    settings = Settings(_env_file=None, chat_rate_limit_requests=2, chat_rate_limit_window_seconds=60)
    monkeypatch.setattr(rate_limit, "get_settings", lambda: settings)
    rate_limit.get_request_limiter.cache_clear()
    requests = []
    def transport(request):
        requests.append(request)
        token = request.headers["Authorization"]
        if token == "Bearer bad": return httpx.Response(401, json={"code": 401})
        if request.url.path == "/api/v1/user/profile":
            return httpx.Response(200, json={"code": 200, "data": {"id": 8 if token == "Bearer other-user" else 7}})
        return httpx.Response(200, json={"code": 200, "data": version_data()})
    for module in (chat, knowledge):
        monkeypatch.setattr(module, "get_settings", lambda: settings)
        monkeypatch.setattr(module, "BlogClient", lambda _: BlogClient(settings, transport=httpx.MockTransport(transport)))
    runner = Mock(run_with_response=AsyncMock(return_value=AgentResponse(answer="ok")))
    factory = Mock(return_value=runner)
    service = Mock(search_published_knowledge=AsyncMock(return_value=[]))
    constructor = Mock(return_value=service)
    monkeypatch.setattr(knowledge, "get_published_service", constructor)
    app = FastAPI()
    app.include_router(chat.router)
    app.include_router(knowledge.router)
    app.include_router(health_router)
    app.dependency_overrides[chat.runner_factory] = lambda: factory
    client = TestClient(app)
    body = {"message": "q", "workspace": {"article_id": 23, "version_no": 7}}
    try:
        assert client.post("/api/v1/agent/chat", headers={"Authorization": "Bearer bad"}, json=body).status_code == 401
        assert client.post("/api/v1/agent/chat", headers={"Authorization": "Bearer jwt-one"}, json=body).status_code == 200
        # 更换 token 不更换 Go 认证后的用户身份，两个入口共享同一额度。
        assert client.post("/api/v1/agent/knowledge/chat", headers={"Authorization": "Bearer jwt-two"}, json={"query": "q"}).status_code == 200
        for path, data in [("/chat", body), ("/knowledge/chat", {"query": "q"})]:
            before = len(requests)
            response = client.post("/api/v1/agent"+path, headers={"Authorization": "Bearer jwt-three"}, json=data)
            assert response.status_code == 429
            assert response.json() == {"detail": {"code": "rate_limit_exceeded", "message": "请求过于频繁，请稍后再试。"}}
            assert 1 <= int(response.headers["Retry-After"]) <= 60
            assert len(requests) == before + 1 and requests[-1].url.path == "/api/v1/user/profile"
        factory.assert_called_once()
        constructor.assert_called_once()
        assert client.get("/health").status_code == 200
        assert client.post("/api/v1/agent/knowledge/chat", headers={"Authorization": "Bearer other-user"}, json={"query": "q"}).status_code == 200
        assert set(rate_limit.get_request_limiter()._buckets) == {("user", 7), ("user", 8)}
    finally:
        rate_limit.get_request_limiter.cache_clear()

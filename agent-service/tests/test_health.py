import importlib

import pytest
from fastapi.testclient import TestClient

from app.core.config import get_settings


@pytest.mark.parametrize("app_name", ["blog-agent", "configured-agent"])
def test_health_and_application_configuration(monkeypatch, tmp_path, app_name):
    # Do not read a developer's real .env or reuse cached settings from another test.
    monkeypatch.chdir(tmp_path)
    monkeypatch.setenv("APP_NAME", app_name)
    monkeypatch.setenv("APP_ENV", "test")
    monkeypatch.setenv("LOG_LEVEL", "INFO")
    get_settings.cache_clear()
    try:
        from app import main

        importlib.reload(main)
        with TestClient(main.app) as client:
            response = client.get("/health")

        assert response.status_code == 200
        assert response.json() == {"status": "ok", "service": app_name}
        assert main.app.title == app_name
        assert main.app.version == "0.1.0"
    finally:
        get_settings.cache_clear()

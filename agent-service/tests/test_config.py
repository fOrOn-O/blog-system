import pytest
from pydantic import ValidationError

from app.core.config import Settings


@pytest.fixture(autouse=True)
def isolate_environment(monkeypatch):
    for name in (
        "APP_NAME", "APP_ENV", "LOG_LEVEL", "BLOG_BACKEND_URL", "BLOG_BACKEND_TIMEOUT_SECONDS"
    ):
        monkeypatch.delenv(name, raising=False)


def test_default_settings():
    settings = Settings(_env_file=None)

    assert settings.app_name == "blog-agent"
    assert settings.app_env == "development"
    assert settings.log_level == "INFO"
    assert str(settings.blog_backend_url) == "http://localhost:8080/"
    assert settings.blog_backend_timeout_seconds == 10


def test_dotenv_settings(tmp_path):
    env_file = tmp_path / ".env"
    env_file.write_text(
        "APP_NAME=local-agent\nAPP_ENV=test\nLOG_LEVEL=DEBUG\n"
        "BLOG_BACKEND_URL=http://blog.test:8081\nBLOG_BACKEND_TIMEOUT_SECONDS=2.5\n",
        encoding="utf-8",
    )

    settings = Settings(_env_file=env_file)

    assert settings.app_name == "local-agent"
    assert settings.app_env == "test"
    assert settings.log_level == "DEBUG"
    assert str(settings.blog_backend_url) == "http://blog.test:8081/"
    assert settings.blog_backend_timeout_seconds == 2.5


def test_environment_overrides_dotenv(tmp_path, monkeypatch):
    env_file = tmp_path / ".env"
    env_file.write_text(
        "APP_NAME=local-agent\nAPP_ENV=development\nLOG_LEVEL=INFO\n"
        "BLOG_BACKEND_URL=http://blog.test:8081\nBLOG_BACKEND_TIMEOUT_SECONDS=2.5\n",
        encoding="utf-8",
    )
    monkeypatch.setenv("APP_NAME", "configured-agent")
    monkeypatch.setenv("APP_ENV", "test")
    monkeypatch.setenv("LOG_LEVEL", "WARNING")
    monkeypatch.setenv("BLOG_BACKEND_URL", "https://blog.example.com")
    monkeypatch.setenv("BLOG_BACKEND_TIMEOUT_SECONDS", "7")

    settings = Settings(_env_file=env_file)

    assert settings.app_name == "configured-agent"
    assert settings.app_env == "test"
    assert settings.log_level == "WARNING"
    assert str(settings.blog_backend_url) == "https://blog.example.com/"
    assert settings.blog_backend_timeout_seconds == 7


def test_invalid_log_level_is_rejected():
    with pytest.raises(ValidationError, match="log_level"):
        Settings(_env_file=None, log_level="INVALID")


@pytest.mark.parametrize("timeout", [0, -1, "nan", "inf"])
def test_backend_timeout_must_be_finite_and_positive(timeout):
    with pytest.raises(ValidationError):
        Settings(_env_file=None, blog_backend_timeout_seconds=timeout)


@pytest.mark.parametrize("url", [
    "not-a-url", "ftp://blog.test", "http://user:password@blog.test",
    "http://blog.test?token=value", "http://blog.test/#fragment",
    "http://blog.test/api/v1/agent",
])
def test_backend_url_must_be_an_http_origin(url):
    with pytest.raises(ValidationError):
        Settings(_env_file=None, blog_backend_url=url)

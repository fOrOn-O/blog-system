import pytest
from pydantic import ValidationError

from app.core.config import Settings


@pytest.fixture(autouse=True)
def isolate_environment(monkeypatch):
    for name in ("APP_NAME", "APP_ENV", "LOG_LEVEL"):
        monkeypatch.delenv(name, raising=False)


def test_default_settings():
    settings = Settings(_env_file=None)

    assert settings.app_name == "blog-agent"
    assert settings.app_env == "development"
    assert settings.log_level == "INFO"


def test_dotenv_settings(tmp_path):
    env_file = tmp_path / ".env"
    env_file.write_text(
        "APP_NAME=local-agent\nAPP_ENV=test\nLOG_LEVEL=DEBUG\n", encoding="utf-8"
    )

    settings = Settings(_env_file=env_file)

    assert settings.app_name == "local-agent"
    assert settings.app_env == "test"
    assert settings.log_level == "DEBUG"


def test_environment_overrides_dotenv(tmp_path, monkeypatch):
    env_file = tmp_path / ".env"
    env_file.write_text(
        "APP_NAME=local-agent\nAPP_ENV=development\nLOG_LEVEL=INFO\n",
        encoding="utf-8",
    )
    monkeypatch.setenv("APP_NAME", "configured-agent")
    monkeypatch.setenv("APP_ENV", "test")
    monkeypatch.setenv("LOG_LEVEL", "WARNING")

    settings = Settings(_env_file=env_file)

    assert settings.app_name == "configured-agent"
    assert settings.app_env == "test"
    assert settings.log_level == "WARNING"


def test_invalid_log_level_is_rejected():
    with pytest.raises(ValidationError, match="log_level"):
        Settings(_env_file=None, log_level="INVALID")

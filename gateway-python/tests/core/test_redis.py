import pytest
import json
from unittest.mock import AsyncMock, MagicMock
from core.config import Settings
import redis.exceptions

def test_redis_url_generation():
    """Valida la generación de REDIS_URL con y sin contraseña."""
    settings_no_pwd = Settings(
        SECRET_KEY="test", DB_USER="u", DB_PASSWORD="p", DB_NAME="d",
        EMAIL_ADMIN="a@a.com", PASSWORD_ADMIN="12345678",
        MICROSERVICES="{}",
        REDIS_HOST="127.0.0.1", REDIS_PORT=6380, REDIS_DB=1, REDIS_PASSWORD=""
    )
    assert settings_no_pwd.REDIS_URL == "redis://127.0.0.1:6380/1"

    settings_with_pwd = Settings(
        SECRET_KEY="test", DB_USER="u", DB_PASSWORD="p", DB_NAME="d",
        EMAIL_ADMIN="a@a.com", PASSWORD_ADMIN="12345678",
        MICROSERVICES="{}",
        REDIS_HOST="redis-server", REDIS_PORT=6379, REDIS_DB=0, REDIS_PASSWORD="secret_pwd"
    )
    assert settings_with_pwd.REDIS_URL == "redis://:secret_pwd@redis-server:6379/0"

@pytest.mark.asyncio
async def test_redis_set_and_get_cached_json(mocker):
    """Valida que set_cached_json y get_cached_json almacenen y recuperen datos estructurados."""
    from core.redis import set_cached_json, get_cached_json, get_redis_client

    mock_client = AsyncMock()
    mock_client.get.return_value = json.dumps({"key": "val", "num": 42}).encode("utf-8")
    mock_client.set.return_value = True

    mocker.patch("core.redis.get_redis_client", return_value=mock_client)

    success = await set_cached_json("test:key", {"key": "val", "num": 42}, ttl=30)
    assert success is True
    mock_client.set.assert_called_once_with("test:key", json.dumps({"key": "val", "num": 42}), ex=30)

    data = await get_cached_json("test:key")
    assert data == {"key": "val", "num": 42}
    mock_client.get.assert_called_once_with("test:key")

@pytest.mark.asyncio
async def test_redis_graceful_fallback_on_error(mocker, caplog):
    """Valida que si Redis falla, no lance excepción sino que haga fallback retornando None."""
    from core.redis import get_cached_json, set_cached_json

    mock_client = AsyncMock()
    mock_client.get.side_effect = redis.exceptions.ConnectionError("Redis down")
    mock_client.set.side_effect = redis.exceptions.ConnectionError("Redis down")

    mocker.patch("core.redis.get_redis_client", return_value=mock_client)

    with caplog.at_level("WARNING"):
        val = await get_cached_json("fail:key")
        assert val is None
        assert "Fallo en operación de Redis" in caplog.text

    with caplog.at_level("WARNING"):
        res = await set_cached_json("fail:key", {"any": 1})
        assert res is False

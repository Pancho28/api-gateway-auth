import pytest
from core.config import Settings

def test_config_parsing():
    """Valida el correcto parseo de las configuraciones y variables de entorno."""
    settings = Settings(
        SECRET_KEY="test_key",
        DB_USER="usr",
        DB_PASSWORD="pwd",
        DB_NAME="db_test",
        EMAIL_ADMIN="admin@test.com",
        PASSWORD_ADMIN="pass",
        MICROSERVICES='{"service1": {"url": "http://serv1", "api_key": "123"}}'
    )
    
    assert "service1" in settings.MICROSERVICES
    assert settings.MICROSERVICES["service1"]["url"] == "http://serv1"
    assert "postgresql+asyncpg://usr:pwd@localhost:5432/db_test" == settings.ASYNC_DATABASE_URL

def test_config_pgbouncer_port():
    """Valida la URL de conexión cuando se apunta al puerto de PgBouncer (6432)."""
    settings = Settings(
        SECRET_KEY="test_key",
        DB_USER="usr",
        DB_PASSWORD="pwd",
        DB_PORT=6432,
        DB_NAME="db_test",
        EMAIL_ADMIN="admin@test.com",
        PASSWORD_ADMIN="pass",
        MICROSERVICES='{}'
    )
    assert "postgresql+asyncpg://usr:pwd@localhost:6432/db_test" == settings.ASYNC_DATABASE_URL

def test_config_invalid_json():
    """Valida que se lance un error al intentar parsear un JSON inválido en MICROSERVICES."""
    with pytest.raises(ValueError, match="JSON string válido"):
        Settings(
            SECRET_KEY="test", DB_USER="1", DB_PASSWORD="1", DB_NAME="1", 
            EMAIL_ADMIN="1", PASSWORD_ADMIN="1",
            MICROSERVICES="{inváalid-json}"
        )

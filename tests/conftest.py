import pytest
import os
from unittest.mock import AsyncMock, MagicMock

# --- Forzar variables de entorno ANTES de importar la app ---
os.environ["SECRET_KEY"] = "test_secret"
os.environ["DB_USER"] = "test"
os.environ["DB_PASSWORD"] = "test"
os.environ["DB_NAME"] = "test"
os.environ["EMAIL_ADMIN"] = "admin@test.com"
os.environ["PASSWORD_ADMIN"] = "testpass"
os.environ["MICROSERVICES"] = '{"test.com": {"url": "http://test", "api_key": "test"}}'

from httpx import AsyncClient, ASGITransport
from main import app
from core.database import get_db

@pytest.fixture
def mock_db_session():
    """Provee una sesión de DB mocker para evitar conectar a BD real."""
    session = AsyncMock()
    
    # Pre-configuración base para cuando se llame a session.execute(stmt)
    # Puede ser sobrescrito en los tests individuales si necesitan retornar usuarios.
    mock_result = MagicMock()
    mock_scalars = MagicMock()
    mock_scalars.first.return_value = None
    mock_result.scalars.return_value = mock_scalars
    
    session.execute.return_value = mock_result
    return session

@pytest.fixture
def override_get_db(mock_db_session):
    """Sobreescribe la dependencia get_db en FastAPI."""
    async def _override_get_db():
        yield mock_db_session
    app.dependency_overrides[get_db] = _override_get_db
    yield
    app.dependency_overrides.clear()

@pytest.fixture
async def async_client(override_get_db):
    """Cliente HTTP asíncrono para testear routers."""
    async with AsyncClient(transport=ASGITransport(app=app), base_url="http://test") as client:
        yield client

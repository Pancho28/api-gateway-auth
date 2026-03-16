import pytest
from unittest.mock import MagicMock
from models.user import User
from api.dependencies import get_current_admin_user
from main import app
from services.user_service import user_service

@pytest.mark.asyncio
async def test_create_user_endpoint_success(mocker, async_client, mock_db_session):
    """Prueba de integración: Valida la creación exitosa de un usuario vía API por un administrador."""
    # Evade el chequeo de JWT real forzando siempre la obtención de un admin en este endpoint
    app.dependency_overrides[get_current_admin_user] = lambda: User(email="admin@admin.com", role="ADMIN")
    
    # Mockear el service para devolver un usuario válido con ID
    mock_user = User(id="1234-5678", email="new_user@test.com", role="USER")
    mocker.patch.object(user_service, 'create_user_by_admin', new_callable=mocker.AsyncMock, return_value=mock_user)
    
    resp = await async_client.post("/users", json={"email": "new_user@test.com", "password":"password123", "role":"USER"})
    
    assert resp.status_code == 201
    assert resp.json()["email"] == "new_user@test.com"
    assert resp.json()["role"] == "USER"
    
    app.dependency_overrides.clear()

@pytest.mark.asyncio
async def test_create_user_endpoint_unauthorized(async_client):
    """Prueba de integración: Valida que el endpoint de creación de usuarios rechace peticiones no autenticadas."""
    # Sin overide de dependencias, un llamado sin auth falla por 401
    resp = await async_client.post("/users", json={"email": "u@test.com", "password":"pwd", "role":"USER"})
    assert resp.status_code == 401

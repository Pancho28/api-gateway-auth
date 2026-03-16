import pytest
from core.security import get_password_hash
from models.user import User
from unittest.mock import MagicMock

@pytest.mark.asyncio
async def test_login_endpoint_success(async_client, mock_db_session):
    """Prueba de integración: Valida el endpoint de login con credenciales correctas."""
    mock_db_user = User(email="t@test.com", hashed_password=get_password_hash("pass"), role="USER")
    
    mock_scalars = MagicMock()
    mock_scalars.first.return_value = mock_db_user
    mock_result = MagicMock()
    mock_result.scalars.return_value = mock_scalars
    mock_db_session.execute.return_value = mock_result
    
    response = await async_client.post("/auth/login", json={"email": "t@test.com", "password": "pass"})
    
    assert response.status_code == 200
    json_resp = response.json()
    assert "token" in json_resp
    assert json_resp["user"] == "t@test.com"
    assert json_resp["role"] == "USER"

@pytest.mark.asyncio
async def test_login_endpoint_unauthorized(async_client, mock_db_session):
    """Prueba de integración: Valida el endpoint de login con credenciales incorrectas."""
    mock_db_user = User(email="t@test.com", hashed_password=get_password_hash("pass"), role="USER")
    
    mock_scalars = MagicMock()
    mock_scalars.first.return_value = mock_db_user
    mock_result = MagicMock()
    mock_result.scalars.return_value = mock_scalars
    mock_db_session.execute.return_value = mock_result
    
    response = await async_client.post("/auth/login", json={"email": "t@test.com", "password": "wrong"})
    assert response.status_code == 401

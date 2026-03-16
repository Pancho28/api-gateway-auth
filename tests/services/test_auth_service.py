import pytest
from fastapi import HTTPException
from unittest.mock import MagicMock
from services.auth_service import auth_service
from api.schemas.user import UserLogin
from models.user import User
from core.security import get_password_hash

@pytest.mark.asyncio
async def test_login_user_success(mock_db_session):
    """Valida que un usuario pueda iniciar sesión exitosamente con credenciales válidas."""
    user_in = UserLogin(email="test@test.com", password="password")
    mock_db_user = User(email="test@test.com", hashed_password=get_password_hash("password"), role="USER")
    
    mock_scalars = MagicMock()
    mock_scalars.first.return_value = mock_db_user
    mock_result = MagicMock()
    mock_result.scalars.return_value = mock_scalars
    mock_db_session.execute.return_value = mock_result
    
    result = await auth_service.login_user(mock_db_session, user_in)
    
    assert result["user"] == "test@test.com"
    assert result["role"] == "USER"
    assert "token" in result

@pytest.mark.asyncio
async def test_login_user_wrong_password(mock_db_session):
    """Valida que se lance un error 401 al intentar iniciar sesión con una contraseña incorrecta."""
    user_in = UserLogin(email="test@test.com", password="wrong_password")
    mock_db_user = User(email="test@test.com", hashed_password=get_password_hash("password"), role="USER")
    
    mock_scalars = MagicMock()
    mock_scalars.first.return_value = mock_db_user
    mock_result = MagicMock()
    mock_result.scalars.return_value = mock_scalars
    mock_db_session.execute.return_value = mock_result
    
    with pytest.raises(HTTPException) as exc_info:
        await auth_service.login_user(mock_db_session, user_in)
    assert exc_info.value.status_code == 401

@pytest.mark.asyncio
async def test_login_user_not_found(mock_db_session):
    """Valida que se lance un error 401 al intentar iniciar sesión con un correo no registrado."""
    user_in = UserLogin(email="test@test.com", password="password")
    
    mock_scalars = MagicMock()
    mock_scalars.first.return_value = None
    mock_result = MagicMock()
    mock_result.scalars.return_value = mock_scalars
    mock_db_session.execute.return_value = mock_result
    
    with pytest.raises(HTTPException) as exc_info:
        await auth_service.login_user(mock_db_session, user_in)
    assert exc_info.value.status_code == 401

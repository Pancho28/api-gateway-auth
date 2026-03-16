import pytest
from fastapi import HTTPException, Request
from unittest.mock import MagicMock
from api.dependencies import get_current_user, get_current_admin_user
from core.security import create_access_token
from models.user import User

@pytest.mark.asyncio
async def test_get_current_user_success(mock_db_session):
    """Valida la obtención exitosa del usuario actual procesando un token JWT válido."""
    token = create_access_token({"sub": "test@test.com", "role": "USER"})
    
    # Mock DB response
    mock_db_user = User(email="test@test.com", role="USER")
    mock_scalars = MagicMock()
    mock_scalars.first.return_value = mock_db_user
    mock_result = MagicMock()
    mock_result.scalars.return_value = mock_scalars
    mock_db_session.execute.return_value = mock_result
    
    req = MagicMock(spec=Request)
    req.state = MagicMock()
    
    user = await get_current_user(request=req, token=token, db=mock_db_session)
    assert user.email == "test@test.com"
    assert req.state.user == mock_db_user

@pytest.mark.asyncio
async def test_get_current_user_invalid_token(mock_db_session):
    """Valida que se lance un error 401 al intentar obtener el usuario con un token inválido."""
    req = MagicMock(spec=Request)
    with pytest.raises(HTTPException) as exc:
        await get_current_user(request=req, token="invalid.token.str", db=mock_db_session)
    assert exc.value.status_code == 401

@pytest.mark.asyncio
async def test_get_current_user_not_found(mock_db_session):
    """Valida que se lance un error 401 cuando el usuario codificado en el token no existe en la base de datos."""
    token = create_access_token({"sub": "none@test.com", "role": "USER"})
    
    mock_scalars = MagicMock()
    mock_scalars.first.return_value = None
    mock_result = MagicMock()
    mock_result.scalars.return_value = mock_scalars
    mock_db_session.execute.return_value = mock_result
    
    req = MagicMock(spec=Request)
    
    with pytest.raises(HTTPException) as exc:
        await get_current_user(request=req, token=token, db=mock_db_session)
    assert exc.value.status_code == 401

@pytest.mark.asyncio
async def test_get_current_admin_user_success():
    """Valida que un usuario con rol ADMIN sea identificado correctamente como administrador."""
    user = User(email="admin@test.com", role="ADMIN")
    admin = await get_current_admin_user(user)
    assert admin == user

@pytest.mark.asyncio
async def test_get_current_admin_user_forbidden():
    """Valida que se lance un error 403 cuando un usuario sin rol ADMIN intenta acceder como administrador."""
    user_normal = User(email="user@test.com", role="USER")
    with pytest.raises(HTTPException) as exc:
        await get_current_admin_user(user_normal)
    assert exc.value.status_code == 403

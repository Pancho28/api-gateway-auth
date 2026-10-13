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

@pytest.mark.asyncio
async def test_get_current_user_redis_cache_hit(mocker, mock_db_session):
    """Valida que get_current_user recupere la sesión desde Redis sin consultar a PostgreSQL (Cache Hit)."""
    token = create_access_token({"sub": "cached@test.com", "role": "USER"})
    cached_session = {"id": "uuid-1234", "email": "cached@test.com", "role": "USER"}
    
    mock_get_cached = mocker.patch("api.dependencies.get_cached_json", new_callable=mocker.AsyncMock, return_value=cached_session)
    
    req = MagicMock(spec=Request)
    req.state = MagicMock()
    
    user = await get_current_user(request=req, token=token, db=mock_db_session)
    
    assert user.email == "cached@test.com"
    assert user.role == "USER"
    assert user.id == "uuid-1234"
    mock_get_cached.assert_called_once_with("session:cached@test.com")
    # Verificar que NO se ejecutó consulta a la base de datos
    mock_db_session.execute.assert_not_called()

@pytest.mark.asyncio
async def test_get_current_user_redis_cache_miss_populates_redis(mocker, mock_db_session):
    """Valida que ante un Cache Miss en Redis, se consulte a PostgreSQL y se repueble Redis."""
    token = create_access_token({"sub": "miss@test.com", "role": "USER"})
    
    mock_get_cached = mocker.patch("api.dependencies.get_cached_json", new_callable=mocker.AsyncMock, return_value=None)
    mock_set_cached = mocker.patch("api.dependencies.set_cached_json", new_callable=mocker.AsyncMock, return_value=True)
    
    mock_db_user = User(id="uuid-5678", email="miss@test.com", role="USER")
    mock_scalars = MagicMock()
    mock_scalars.first.return_value = mock_db_user
    mock_result = MagicMock()
    mock_result.scalars.return_value = mock_scalars
    mock_db_session.execute.return_value = mock_result
    
    req = MagicMock(spec=Request)
    req.state = MagicMock()
    
    user = await get_current_user(request=req, token=token, db=mock_db_session)
    
    assert user.email == "miss@test.com"
    mock_get_cached.assert_called_once_with("session:miss@test.com")
    mock_db_session.execute.assert_called_once()
    mock_set_cached.assert_called_once_with("session:miss@test.com", {"id": "uuid-5678", "email": "miss@test.com", "role": "USER"}, ttl=1800)


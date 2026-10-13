import pytest
from fastapi import HTTPException
from sqlalchemy.exc import IntegrityError
from unittest.mock import MagicMock
from services.user_service import user_service
from api.schemas.user import AdminUserCreate
from models.user import User

@pytest.mark.asyncio
async def test_create_user_by_admin_success(mock_db_session):
    """Valida la creación exitosa de un usuario por parte de un administrador."""
    user_in = AdminUserCreate(email="new@test.com", password="password123", role="ADMIN")
    
    # Ensure user does not exist
    mock_scalars = MagicMock()
    mock_scalars.first.return_value = None
    mock_result = MagicMock()
    mock_result.scalars.return_value = mock_scalars
    mock_db_session.execute.return_value = mock_result
    
    result = await user_service.create_user_by_admin(mock_db_session, user_in)
    
    assert result.email == "new@test.com"
    assert result.role == "ADMIN"
    mock_db_session.add.assert_called_once()
    mock_db_session.commit.assert_called_once()

@pytest.mark.asyncio
async def test_create_user_already_exists(mock_db_session):
    """Valida que se lance un error 400 al intentar crear un usuario con un email ya registrado."""
    user_in = AdminUserCreate(email="exists@test.com", password="password123", role="USER")
    
    mock_scalars = MagicMock()
    mock_scalars.first.return_value = User(email="exists@test.com")
    mock_result = MagicMock()
    mock_result.scalars.return_value = mock_scalars
    mock_db_session.execute.return_value = mock_result
    
    with pytest.raises(HTTPException) as exc_info:
        await user_service.create_user_by_admin(mock_db_session, user_in)
    assert exc_info.value.status_code == 400
    assert "ya está registrado" in exc_info.value.detail

@pytest.mark.asyncio
async def test_create_user_integrity_error(mock_db_session, caplog):
    """Valida el manejo de errores de integridad de la base de datos durante la creación de un usuario."""
    user_in = AdminUserCreate(email="err@test.com", password="pwd12345", role="USER")
    
    mock_scalars = MagicMock()
    mock_scalars.first.return_value = None
    mock_result = MagicMock()
    mock_result.scalars.return_value = mock_scalars
    mock_db_session.execute.return_value = mock_result
    
    mock_db_session.commit.side_effect = IntegrityError("STATEMENT", "PARAMS", Exception("SENSITIVE_TABLE_SCHEMA_INFO"))
    
    with caplog.at_level("ERROR"):
        with pytest.raises(HTTPException) as exc_info:
            await user_service.create_user_by_admin(mock_db_session, user_in)
        
    assert exc_info.value.status_code == 400
    assert "SENSITIVE_TABLE_SCHEMA_INFO" not in caplog.text
    mock_db_session.rollback.assert_called_once()

@pytest.mark.asyncio
async def test_create_user_general_error(mock_db_session, caplog):
    """Valida el manejo de excepciones generales durante la creación de un usuario."""
    user_in = AdminUserCreate(email="err@test.com", password="password123", role="USER")
    mock_scalars = MagicMock()
    mock_scalars.first.return_value = None
    mock_result = MagicMock()
    mock_result.scalars.return_value = mock_scalars
    mock_db_session.execute.return_value = mock_result
    
    mock_db_session.commit.side_effect = Exception("SENSITIVE_DB_CREDENTIALS_LEAK")
    
    with caplog.at_level("CRITICAL"):
        with pytest.raises(HTTPException) as exc_info:
            await user_service.create_user_by_admin(mock_db_session, user_in)
        
    assert exc_info.value.status_code == 500
    assert "SENSITIVE_DB_CREDENTIALS_LEAK" not in caplog.text
    mock_db_session.rollback.assert_called_once()

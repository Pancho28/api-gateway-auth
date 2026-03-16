import pytest
from models.user import User
from api.dependencies import get_current_user
from services.proxy_service import proxy_service
from main import app
from fastapi.responses import JSONResponse

@pytest.mark.asyncio
async def test_health_check(async_client):
    """Prueba de integración: Valida el endpoint de health check público."""
    resp = await async_client.get("/health")
    assert resp.status_code == 200
    assert resp.json()["status"] == "healthy"

@pytest.mark.asyncio
async def test_reverse_proxy_unauthorized(async_client):
    """Prueba de integración: Valida que el proxy rechace peticiones sin autenticación."""
    # Cualquier ruta interceptada falla si no hay Authorization Bearer Token
    resp = await async_client.get("/alguna-ruta-protegida")
    assert resp.status_code == 401

@pytest.mark.asyncio
async def test_reverse_proxy_authorized_forward(async_client, mocker):
    """Prueba de integración: Valida el reenvío exitoso a través del proxy para usuarios autenticados.""" 
    app.dependency_overrides[get_current_user] = lambda: User(email="user@test.com", role="USER")
    
    mocker.patch.object(proxy_service, 'forward_request', return_value=JSONResponse({"mocked": True}))
    
    resp = await async_client.get("/testing/ruta", headers={"host": "test.com"})
    
    assert resp.status_code == 200
    assert resp.json() == {"mocked": True}
    proxy_service.forward_request.assert_called_once()
    
    app.dependency_overrides.clear()

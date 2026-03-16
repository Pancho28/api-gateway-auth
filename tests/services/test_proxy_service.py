import pytest
from fastapi import Request
from unittest.mock import MagicMock
from services.proxy_service import proxy_service
from fastapi import HTTPException
import httpx

@pytest.mark.asyncio
async def test_proxy_service_no_config():
    """Valida que se lance un error 404 cuando se intenta acceder a un host no configurado en MICROSERVICES."""
    req = MagicMock(spec=Request)
    req.headers = {"host": "unknown.com"}
    req.method = "GET"
    req.query_params = {}
    
    with pytest.raises(HTTPException) as exc_info:
        await proxy_service.forward_request(req, "test_path")
    assert exc_info.value.status_code == 404

@pytest.mark.asyncio
async def test_proxy_service_forward_success(mocker):
    """Valida el reenvío exitoso de una petición a un microservicio configurado."""
    # En pytest.ini cargamos MICROSERVICES={"test.com": {"url": "http://test", "api_key": "test"}}
    req = MagicMock(spec=Request)
    req.headers = {"host": "test.com", "other-header": "value"}
    req.method = "POST"
    req.query_params = {"q": "search"}
    
    # Mockear request.body()
    asyn_body = mocker.AsyncMock(return_value=b'payload')
    req.body = asyn_body
    
    # Mockear client.request de HTTPX
    mock_resp = MagicMock()
    mock_resp.status_code = 200
    mock_resp.headers = {"content-type": "application/json"}
    mock_resp.json.return_value = {"msg": "success"}
    
    mocker.patch.object(proxy_service.client, 'request', new_callable=mocker.AsyncMock, return_value=mock_resp)
    
    resp = await proxy_service.forward_request(req, "api/data")
    
    assert resp.status_code == 200
    import json
    assert json.loads(resp.body) == {"msg": "success"}
    
    # Asegurar que se llamó correctamente a httpx
    proxy_service.client.request.assert_called_once_with(
        method="POST",
        url="http://test/api/data",
        headers={"other-header": "value", "X-Internal-Api-Key": "test"},
        params=req.query_params,
        content=b'payload'
    )

@pytest.mark.asyncio
async def test_proxy_service_httpx_exception(mocker):
    """Valida que las excepciones de HTTPX se propaguen correctamente durante el reenvío."""
    req = MagicMock(spec=Request)
    req.headers = {"host": "test.com"}
    req.method = "GET"
    req.query_params = {}
    req.body = mocker.AsyncMock(return_value=b'')
    
    mocker.patch.object(proxy_service.client, 'request', new_callable=mocker.AsyncMock, side_effect=httpx.RequestError("Timeout"))
    
    # El proxy deja que las excepciones HTTPX las atrape el Exception Handler global
    with pytest.raises(httpx.RequestError):
        await proxy_service.forward_request(req, "api/data")

@pytest.mark.asyncio
async def test_proxy_service_close_client(mocker):
    """Valida el cierre correcto del cliente HTTPX."""
    mocker.patch.object(proxy_service.client, 'aclose', new_callable=mocker.AsyncMock)
    await proxy_service.close()
    proxy_service.client.aclose.assert_called_once()

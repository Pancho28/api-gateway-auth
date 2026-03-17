import pytest
from fastapi import Request
from unittest.mock import MagicMock
from services.proxy_service import proxy_service
from fastapi import HTTPException
import httpx

@pytest.mark.asyncio
async def test_proxy_service_no_config():
    """Valida que se lance un error 404 cuando se intenta acceder al Gateway sin un prefijo válido."""
    req = MagicMock(spec=Request)
    req.method = "GET"
    req.query_params = {}
    
    # "unknown/path" -> prefix="unknown", no configurado
    with pytest.raises(HTTPException) as exc_info:
        await proxy_service.forward_request(req, "unknown/path")
    assert exc_info.value.status_code == 404

@pytest.mark.asyncio
async def test_proxy_service_forward_success(mocker):
    """Valida el reenvío exitoso de una petición usando prefijos de ruta."""
    # En conftest cargamos MICROSERVICES={"test": {"url": "http://test", "api_key": "test"}}
    req = MagicMock(spec=Request)
    req.headers = {"other-header": "value"}
    req.method = "POST"
    req.query_params = {"q": "search"}
    
    # Mockear request.stream() (iterador asíncrono)
    async def mock_stream():
        yield b'payload'
    req.stream = mock_stream
    
    # Mockear build_request y send de HTTPX
    mock_req = MagicMock()
    mock_resp = MagicMock()
    mock_resp.status_code = 200
    mock_resp.headers = {"content-type": "application/json"}
    
    # aiter_raw debe devolver un iterable (puede ser síncrono si mocked así, 
    # pero mejor ser explícitos con un generador asíncrono)
    async def mock_aiter_raw():
        yield b'{"msg": "success"}'
    
    mock_resp.aiter_raw = mock_aiter_raw
    
    mocker.patch.object(proxy_service.client, 'build_request', return_value=mock_req)
    mocker.patch.object(proxy_service.client, 'send', new_callable=mocker.AsyncMock, return_value=mock_resp)
    
    # "test/api/data" -> prefix="test", sub_path="api/data"
    resp = await proxy_service.forward_request(req, "test/api/data")
    
    assert resp.status_code == 200
    # StreamingResponse tiene un iterador en .body_iterator o .body
    # Para verificar el contenido en tests de FastAPI/Starlette StreamingResponse:
    chunks = [chunk async for chunk in resp.body_iterator]
    assert b"".join(chunks) == b'{"msg": "success"}'
    
    # Verificar que se construyó el request correctamente
    proxy_service.client.build_request.assert_called_once()
    args, kwargs = proxy_service.client.build_request.call_args
    assert kwargs['url'] == "http://test/api/data"
    assert kwargs['headers']['X-Internal-Api-Key'] == "test"

@pytest.mark.asyncio
async def test_proxy_service_httpx_exception(mocker):
    """Valida que las excepciones de HTTPX se propaguen correctamente."""
    req = MagicMock(spec=Request)
    req.method = "GET"
    req.query_params = {}
    req.stream = MagicMock()
    
    mocker.patch.object(proxy_service.client, 'build_request', return_value=MagicMock())
    mocker.patch.object(proxy_service.client, 'send', new_callable=mocker.AsyncMock, side_effect=httpx.ConnectError("Timeout"))
    
    with pytest.raises(HTTPException) as exc_info:
        await proxy_service.forward_request(req, "test/api/data")
    assert exc_info.value.status_code == 502

@pytest.mark.asyncio
async def test_proxy_service_close_client(mocker):
    """Valida el cierre correcto del cliente HTTPX."""
    mocker.patch.object(proxy_service.client, 'aclose', new_callable=mocker.AsyncMock)
    await proxy_service.close()
    proxy_service.client.aclose.assert_called_once()

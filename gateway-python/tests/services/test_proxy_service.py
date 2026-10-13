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

@pytest.mark.asyncio
@pytest.mark.parametrize("malicious_path", [
    "test/../admin",
    "test/%2e%2e/admin",
    "test/%2E%2E/admin",
    "test/./config",
    "test/%2e/config",
    "../test/api",
    "test/api/..\x00",
])
async def test_proxy_service_path_traversal_blocked(malicious_path):
    """Valida que intentos de path traversal sean bloqueados con HTTP 400."""
    req = MagicMock(spec=Request)
    req.method = "GET"
    req.query_params = {}

    with pytest.raises(HTTPException) as exc_info:
        await proxy_service.forward_request(req, malicious_path)
    assert exc_info.value.status_code == 400
    assert "inválida" in exc_info.value.detail or "traversal" in exc_info.value.detail

@pytest.mark.asyncio
async def test_proxy_service_header_stripping(mocker):
    """Valida la purga estricta de cabeceras hop-by-hop y de confianza (anti-spoofing)."""
    req = MagicMock(spec=Request)
    # Headers inyectados maliciosamente por el cliente
    req.headers = {
        "x-internal-api-key": "malicious_spoofed_key",
        "X-Internal-API-Key": "malicious_spoofed_key_2",
        "x-user-role": "SUPERADMIN",
        "X-User-Id": "123",
        "connection": "keep-alive",
        "transfer-encoding": "chunked",
        "upgrade": "websocket",
        "x-legit-client-header": "allowed_value",
    }
    req.method = "GET"
    req.query_params = {}
    
    async def mock_stream():
        yield b''
    req.stream = mock_stream

    mock_req = MagicMock()
    mock_resp = MagicMock()
    mock_resp.status_code = 200
    mock_resp.headers = {}
    
    async def mock_aiter_raw():
        yield b'ok'
    mock_resp.aiter_raw = mock_aiter_raw

    mocker.patch.object(proxy_service.client, 'build_request', return_value=mock_req)
    mocker.patch.object(proxy_service.client, 'send', new_callable=mocker.AsyncMock, return_value=mock_resp)

    await proxy_service.forward_request(req, "test/api/data")

    proxy_service.client.build_request.assert_called_once()
    _, kwargs = proxy_service.client.build_request.call_args
    sent_headers = kwargs['headers']

    # La única clave interna debe ser la legítima de la configuración ("test" en conftest)
    assert sent_headers.get("X-Internal-Api-Key") == "test"
    
    # Verificar que ninguna cabecera prohibida o spoofed pasó al microservicio
    sent_headers_lower = {k.lower(): v for k, v in sent_headers.items()}
    assert "connection" not in sent_headers_lower
    assert "transfer-encoding" not in sent_headers_lower
    assert "upgrade" not in sent_headers_lower
    assert "x-user-role" not in sent_headers_lower
    assert "x-user-id" not in sent_headers_lower
    # La cabecera legítima debe mantenerse
    assert sent_headers.get("x-legit-client-header") == "allowed_value"

@pytest.mark.asyncio
async def test_proxy_service_cache_hit(mocker):
    """Valida que una petición GET con Cache Hit en Redis responda directamente sin contactar al microservicio."""
    req = MagicMock(spec=Request)
    req.method = "GET"
    req.query_params = {"page": "1"}
    req.headers = {}

    cached_payload = {
        "body": '{"data": "from_cache"}',
        "status_code": 200,
        "headers": {"content-type": "application/json"}
    }
    mock_get_cached = mocker.patch("services.proxy_service.get_cached_json", new_callable=mocker.AsyncMock, return_value=cached_payload)
    mock_send = mocker.patch.object(proxy_service.client, 'send', new_callable=mocker.AsyncMock)

    resp = await proxy_service.forward_request(req, "test/api/items")

    assert resp.status_code == 200
    assert resp.headers["X-Cache"] == "HIT"
    assert resp.body == b'{"data": "from_cache"}'
    mock_get_cached.assert_called_once()
    mock_send.assert_not_called()

@pytest.mark.asyncio
async def test_proxy_service_cache_miss_stores_response(mocker):
    """Valida que un Cache Miss ejecute la petición al microservicio y almacene el resultado en Redis."""
    req = MagicMock(spec=Request)
    req.method = "GET"
    req.query_params = {}
    req.headers = {}
    
    async def mock_stream():
        yield b''
    req.stream = mock_stream

    mock_get_cached = mocker.patch("services.proxy_service.get_cached_json", new_callable=mocker.AsyncMock, return_value=None)
    mock_set_cached = mocker.patch("services.proxy_service.set_cached_json", new_callable=mocker.AsyncMock, return_value=True)

    mock_resp = MagicMock()
    mock_resp.status_code = 200
    mock_resp.headers = {"content-type": "application/json"}
    mock_resp.aread = mocker.AsyncMock(return_value=b'{"fresh": true}')

    mocker.patch.object(proxy_service.client, 'build_request', return_value=MagicMock())
    mocker.patch.object(proxy_service.client, 'send', new_callable=mocker.AsyncMock, return_value=mock_resp)

    resp = await proxy_service.forward_request(req, "test/api/items")

    assert resp.status_code == 200
    assert resp.headers["X-Cache"] == "MISS"
    assert resp.body == b'{"fresh": true}'
    mock_get_cached.assert_called_once()
    mock_set_cached.assert_called_once()
    cache_key, cache_val = mock_set_cached.call_args[0][:2]
    assert cache_key.startswith("proxy_cache:test:")
    assert cache_val["body"] == '{"fresh": true}'


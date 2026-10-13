import pytest
from unittest.mock import MagicMock
from fastapi import Request
from main import global_exception_handler, gateway_error_handler
import httpx

@pytest.mark.asyncio
async def test_global_exception_handler_sanitizes_logs(caplog):
    """Valida que el manejador 500 no exponga secretos o cadenas de conexión en logs."""
    sensitive_connection_string = "mysql+aiomysql://admin:SUPER_SECRET_PASSWORD_123@localhost:3306/db"
    sensitive_exc = Exception(f"Database connection failed: {sensitive_connection_string}")
    
    req = MagicMock(spec=Request)
    req.method = "GET"
    req.url.path = "/api/secure-data"

    with caplog.at_level("CRITICAL"):
        response = await global_exception_handler(req, sensitive_exc)

    assert response.status_code == 500
    assert response.headers.get("X-Error-ID") is not None

    # Verificar que el log no contenga la contraseña ni la cadena de conexión sensible
    assert "SUPER_SECRET_PASSWORD_123" not in caplog.text
    assert "mysql+aiomysql" not in caplog.text
    # Verificar que el log mencione el tipo de excepción y el ErrorID
    assert "Exception" in caplog.text
    assert "ErrorID:" in caplog.text

@pytest.mark.asyncio
async def test_gateway_error_handler_sanitizes_logs(caplog):
    """Valida que los errores de microservicios (502) no expongan URLs sensibles en logs."""
    sensitive_url = "http://internal-service.local?secret_token=TOP_SECRET_TOKEN"
    req_mock = MagicMock(spec=Request)
    req_mock.method = "POST"
    req_mock.url.path = "/ventas/checkout"

    exc = httpx.ConnectError(f"Failed to connect to {sensitive_url}")

    with caplog.at_level("ERROR"):
        response = await gateway_error_handler(req_mock, exc)

    assert response.status_code == 502
    assert "TOP_SECRET_TOKEN" not in caplog.text

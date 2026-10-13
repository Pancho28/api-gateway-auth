from fastapi import Request
from unittest.mock import MagicMock
from core.rate_limit import custom_key_func

def test_custom_key_func_with_user():
    """Valida que el rate limiter use el email del usuario cuando está autenticado."""
    req = MagicMock(spec=Request)
    req.state.user = MagicMock()
    req.state.user.email = "test@user.com"
    
    key = custom_key_func(req)
    assert key == "test@user.com"

def test_custom_key_func_fallback_ip(mocker):
    """Valida que el rate limiter use la IP del cliente como fallback cuando no hay usuario."""
    req = MagicMock(spec=Request)
    # Simulamos que no hay usuario seteado
    del req.state.user
    
    req.client = MagicMock()
    req.client.host = "192.168.1.100"
    
    key = custom_key_func(req)
    assert key == "192.168.1.100"

def test_limiter_uses_redis_storage(mocker):
    """Valida que get_rate_limiter_storage_uri retorne REDIS_URL si Redis está disponible o memory:// si no."""
    from core.rate_limit import get_rate_limiter_storage_uri, limiter
    from core.config import settings

    # Si el socket conecta exitosamente -> retorna REDIS_URL
    mocker.patch("socket.create_connection", return_value=MagicMock())
    assert get_rate_limiter_storage_uri() == settings.REDIS_URL

    # Si el socket falla -> degradación elegante a memory://
    mocker.patch("socket.create_connection", side_effect=ConnectionRefusedError())
    assert get_rate_limiter_storage_uri() == "memory://"

    assert limiter._swallow_errors is True

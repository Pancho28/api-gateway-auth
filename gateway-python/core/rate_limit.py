import socket
from fastapi import Request
from slowapi import Limiter
from slowapi.util import get_remote_address

from core.config import settings
from core.logger import get_logger

logger = get_logger(__name__)

def custom_key_func(request: Request) -> str:
    """
    Retorna la clave para el rate_limiter.
    Si el usuario está en el estado de la petición, usa su email u ID.
    Si es una ruta pública, hace fallback a la IP remota.
    """
    if hasattr(request.state, "user") and request.state.user is not None:
        return str(request.state.user.email)
    return get_remote_address(request)

def get_rate_limiter_storage_uri() -> str:
    """
    Determina el URI de almacenamiento para el rate limiter.
    Prioriza Redis (ADR-002) y conmuta a 'memory://' con degradación elegante si Redis no está disponible.
    """
    try:
        with socket.create_connection((settings.REDIS_HOST, settings.REDIS_PORT), timeout=0.1):
            return settings.REDIS_URL
    except (OSError, ConnectionRefusedError):
        logger.info("Redis no detectado en el host configurado. Conmutando rate limiter a storage en memoria local.")
        return "memory://"

# Rate limiter distribuido con backend de Redis y tolerancia a fallos (ADR-002)
limiter = Limiter(
    key_func=custom_key_func,
    storage_uri=get_rate_limiter_storage_uri(),
    swallow_errors=True
)

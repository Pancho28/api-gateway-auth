from fastapi import Request
from slowapi import Limiter
from slowapi.util import get_remote_address

def custom_key_func(request: Request) -> str:
    """
    Retorna la clave para el rate_limiter.
    Si el usuario está en el estado de la petición, usa su email u ID.
    Si es una ruta pública, hace fallback a la IP remota.
    """
    if hasattr(request.state, "user") and request.state.user is not None:
        return str(request.state.user.email)
    return get_remote_address(request)

limiter = Limiter(key_func=custom_key_func)

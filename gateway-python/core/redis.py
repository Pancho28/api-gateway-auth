import json
from typing import Any, Optional
import redis.asyncio as aioredis
from core.config import settings
from core.logger import get_logger

logger = get_logger(__name__)

# Pool de conexiones asíncrono singleton para Redis
_redis_pool: Optional[aioredis.ConnectionPool] = None

def get_redis_pool() -> aioredis.ConnectionPool:
    """Obtiene o inicializa el ConnectionPool asíncrono para Redis."""
    global _redis_pool
    if _redis_pool is None:
        _redis_pool = aioredis.ConnectionPool.from_url(
            settings.REDIS_URL,
            decode_responses=False,
            max_connections=50
        )
    return _redis_pool

def get_redis_client() -> aioredis.Redis:
    """Retorna una instancia de cliente de Redis ligada al pool de conexiones."""
    pool = get_redis_pool()
    return aioredis.Redis(connection_pool=pool)

async def get_cached_json(key: str) -> Optional[Any]:
    """
    Obtiene y deserializa un objeto JSON desde Redis.
    Aplica degradación elegante (Graceful Degradation): retorna None si Redis falla.
    """
    try:
        client = get_redis_client()
        val = await client.get(key)
        if val is not None:
            return json.loads(val.decode("utf-8") if isinstance(val, bytes) else val)
        return None
    except Exception as e:
        logger.warning(f"Fallo en operación de Redis al obtener '{key}': {str(e)}")
        return None

async def set_cached_json(key: str, value: Any, ttl: Optional[int] = None) -> bool:
    """
    Serializa y almacena un objeto JSON en Redis con tiempo de expiración (TTL).
    Retorna True si fue exitoso, o False en caso de degradación por error.
    """
    try:
        client = get_redis_client()
        payload = json.dumps(value)
        await client.set(key, payload, ex=ttl)
        return True
    except Exception as e:
        logger.warning(f"Fallo en operación de Redis al guardar '{key}': {str(e)}")
        return False

async def delete_cached_key(key: str) -> bool:
    """Elimina una clave de Redis de forma segura (ej. invalidación de sesión)."""
    try:
        client = get_redis_client()
        await client.delete(key)
        return True
    except Exception as e:
        logger.warning(f"Fallo en operación de Redis al eliminar '{key}': {str(e)}")
        return False

async def close_redis() -> None:
    """Cierra las conexiones del pool de Redis al apagar la aplicación."""
    global _redis_pool
    if _redis_pool is not None:
        await _redis_pool.disconnect()
        _redis_pool = None

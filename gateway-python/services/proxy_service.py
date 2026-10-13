from urllib.parse import unquote
import hashlib
import httpx
import asyncio
from fastapi import Request, HTTPException, status
from fastapi.responses import Response, StreamingResponse
from core.config import settings
from core.redis import get_cached_json, set_cached_json

# Cabeceras prohibidas: hop-by-hop (RFC 7230) y cabeceras de confianza interna
BLOCKED_HEADERS = {
    # Cabeceras hop-by-hop y de transporte
    "host",
    "content-length",
    "connection",
    "keep-alive",
    "proxy-authenticate",
    "proxy-authorization",
    "te",
    "trailer",
    "trailers",
    "transfer-encoding",
    "upgrade",
    # Cabeceras internas sensibles contra spoofing
    "x-internal-api-key",
    "x-user-id",
    "x-user-email",
    "x-user-role",
}

class ProxyService:
    """Lógica de negocio para el reenvío de peticiones (Proxy) con aceleración en caché."""
    
    def __init__(self):
        self.client = httpx.AsyncClient(timeout=httpx.Timeout(10.0))

    async def forward_request(self, request: Request, path: str):
        """Redirige la petición al microservicio correspondiente sanitizando ruta, cabeceras y cacheando respuestas GET."""
        # 1. Sanitización de ruta contra ataques de Path Traversal
        unquoted_path = unquote(path)

        # Detectar caracteres nulos
        if "\x00" in unquoted_path:
            raise HTTPException(
                status_code=status.HTTP_400_BAD_REQUEST,
                detail="Ruta de acceso inválida o no permitida"
            )

        # Extraer segmentos de la ruta
        path_segments = [seg for seg in unquoted_path.strip("/").split("/") if seg]
        
        if not path_segments:
            raise HTTPException(
                status_code=status.HTTP_400_BAD_REQUEST,
                detail="Ruta de microservicio no especificada"
            )

        # Validar que ningún segmento intente navegación relativa
        for segment in path_segments:
            if segment in ("..", ".") or segment.startswith("..") or segment.endswith(".."):
                raise HTTPException(
                    status_code=status.HTTP_400_BAD_REQUEST,
                    detail="Ruta de acceso inválida: intento de path traversal detectado"
                )
            
        prefix = path_segments[0]
        sub_path = "/".join(path_segments[1:]) if len(path_segments) > 1 else ""
        service_config = settings.MICROSERVICES.get(prefix)

        if not service_config:
            raise HTTPException(
                status_code=status.HTTP_404_NOT_FOUND,
                detail=f"No se encontró configuración para el servicio: {prefix}"
            )

        target_url = service_config.get("url")
        internal_key = service_config.get("api_key")

        if not target_url:
            raise HTTPException(
                status_code=status.HTTP_500_INTERNAL_SERVER_ERROR,
                detail=f"Configuración incompleta para el servicio '{prefix}': falta URL"
            )

        # Construir la URL final sin duplicar slashes
        base_url = target_url.rstrip("/")
        url = f"{base_url}/{sub_path}" if sub_path else base_url
        
        # 2. Interceptación de Caché en Redis para peticiones GET (ADR-002)
        is_get_request = request.method.upper() == "GET"
        cache_key = None

        if is_get_request:
            query_str = str(sorted(request.query_params.items()))
            hash_digest = hashlib.sha256(f"{sub_path}?{query_str}".encode("utf-8")).hexdigest()
            cache_key = f"proxy_cache:{prefix}:{hash_digest}"

            cached_data = await get_cached_json(cache_key)
            if cached_data and isinstance(cached_data, dict):
                resp_headers = dict(cached_data.get("headers", {}))
                resp_headers["X-Cache"] = "HIT"
                resp_headers.pop("content-length", None)
                body_content = cached_data["body"]
                if isinstance(body_content, str):
                    body_content = body_content.encode("utf-8")
                return Response(
                    content=body_content,
                    status_code=cached_data["status_code"],
                    headers=resp_headers
                )

        # 3. Purga estricta de cabeceras (case-insensitive)
        headers = {
            k: v for k, v in request.headers.items()
            if k.lower() not in BLOCKED_HEADERS
        }

        # Inyección autorizada y controlada de la API Key interna
        if internal_key:
            headers["X-Internal-Api-Key"] = internal_key

        # Configuración de reintentos solo para peticiones GET (idempotentes)
        max_retries = 2 if is_get_request else 0
        
        for attempt in range(max_retries + 1):
            try:
                # Creamos el request de httpx con streaming de cuerpo
                req = self.client.build_request(
                    method=request.method,
                    url=url,
                    headers=headers,
                    params=request.query_params,
                    content=request.stream()
                )
                
                # Enviamos el request con stream=True
                resp = await self.client.send(req, stream=True)
                
                # Para peticiones GET: almacenar en caché y retornar con X-Cache: MISS
                if is_get_request:
                    if hasattr(resp, "aread") and callable(resp.aread):
                        read_res = resp.aread()
                        if asyncio.iscoroutine(read_res):
                            body_bytes = await read_res
                        else:
                            body_bytes = getattr(resp, "content", b"")
                    elif hasattr(resp, "aiter_raw"):
                        body_bytes = b"".join([chunk async for chunk in resp.aiter_raw()])
                    else:
                        body_bytes = b""
                    resp_headers = dict(resp.headers)
                    resp_headers["X-Cache"] = "MISS"
                    resp_headers.pop("content-length", None)

                    if resp.status_code == 200 and cache_key:
                        await set_cached_json(
                            cache_key,
                            {
                                "body": body_bytes.decode("utf-8", errors="replace"),
                                "status_code": resp.status_code,
                                "headers": dict(resp.headers),
                            },
                            ttl=settings.REDIS_CACHE_TTL
                        )

                    return Response(
                        content=body_bytes,
                        status_code=resp.status_code,
                        headers=resp_headers
                    )

                # Para peticiones mutables (POST/PUT/DELETE): retorno streaming directo
                return StreamingResponse(
                    resp.aiter_raw(),
                    status_code=resp.status_code,
                    headers=dict(resp.headers),
                    background=None
                )
            except (httpx.ConnectError, httpx.TimeoutException) as e:
                if attempt == max_retries:
                    raise HTTPException(
                        status_code=status.HTTP_502_BAD_GATEWAY,
                        detail=f"No se pudo conectar con el microservicio '{prefix}' tras varios intentos"
                    )
                # Breve pausa antes del siguiente reintento
                await asyncio.sleep(1)
            except Exception as e:
                raise e

    async def close(self):
        """Cierra el cliente HTTP."""
        await self.client.aclose()

proxy_service = ProxyService()

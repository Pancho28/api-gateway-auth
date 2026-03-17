import httpx
import asyncio
from fastapi import Request, HTTPException, status
from fastapi.responses import JSONResponse, StreamingResponse
from core.config import settings

class ProxyService:
    """Lógica de negocio para el reenvío de peticiones (Proxy)."""
    
    def __init__(self):
        self.client = httpx.AsyncClient()

    async def forward_request(self, request: Request, path: str):
        """Redirige la petición al microservicio correspondiente usando prefijos de ruta y streaming."""
        # Extraer el primer segmento de la ruta para identificar el microservicio
        # Ejemplo: "/ventas/productos" -> path puede llegar como "ventas/productos"
        path_segments = path.strip("/").split("/")
        
        if not path_segments or not path_segments[0]:
            raise HTTPException(
                status_code=status.HTTP_400_BAD_REQUEST,
                detail="Ruta de microservicio no especificada"
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
        
        headers = dict(request.headers)
        # Limpiar cabeceras que pueden causar conflictos en el proxy
        headers.pop("host", None)
        headers.pop("content-length", None)

        # Inyección de la API Key interna
        if internal_key:
            headers["X-Internal-Api-Key"] = internal_key

        # Configuración de reintentos solo para peticiones GET (idempotentes)
        max_retries = 2 if request.method.upper() == "GET" else 0
        
        for attempt in range(max_retries + 1):
            try:
                # Creamos el request de httpx
                # Usamos request.stream() para no cargar el cuerpo en memoria si es grande
                req = self.client.build_request(
                    method=request.method,
                    url=url,
                    headers=headers,
                    params=request.query_params,
                    content=request.stream()
                )
                
                # Enviamos el request con stream=True para streaming de respuesta
                resp = await self.client.send(req, stream=True, timeout=httpx.Timeout(10.0))
                
                # Retornamos el stream directamente envuelto en StreamingResponse de FastAPI
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

import httpx
import asyncio
from fastapi import Request, HTTPException, status
from fastapi.responses import JSONResponse
from core.config import settings

class ProxyService:
    """Lógica de negocio para el reenvío de peticiones (Proxy)."""
    
    def __init__(self):
        self.client = httpx.AsyncClient()

    async def forward_request(self, request: Request, path: str):
        """Redirige la petición al microservicio correspondiente."""
        host_header = request.headers.get("host", "")
        service_config = settings.MICROSERVICES.get(host_header)

        if not service_config:
            raise HTTPException(
                status_code=status.HTTP_404_NOT_FOUND,
                detail=f"No se encontró configuración para el host: {host_header}"
            )

        target_url = service_config.get("url")
        internal_key = service_config.get("api_key")

        if not target_url:
            raise HTTPException(
                status_code=status.HTTP_500_INTERNAL_SERVER_ERROR,
                detail="Configuración de microservicio incompleta: falta URL"
            )

        url = f"{target_url.rstrip('/')}/{path}"
        
        headers = dict(request.headers)
        headers.pop("host", None)

        # Inyección de la API Key interna desde la configuración unificada
        if internal_key:
            headers["X-Internal-Api-Key"] = internal_key

        try:
            content = await request.body()
            timeout = httpx.Timeout(3.0) # 3 segundos de timeout
            
            # Solo permitir reintentos en peticiones GET
            max_retries = 2 if request.method.upper() == "GET" else 0
            
            for attempt in range(max_retries + 1):
                try:
                    resp = await self.client.request(
                        method=request.method,
                        url=url,
                        headers=headers,
                        params=request.query_params,
                        content=content,
                        timeout=timeout
                    )
                    
                    return JSONResponse(
                        status_code=resp.status_code,
                        content=resp.json() if resp.headers.get("content-type") == "application/json" else resp.text,
                        headers=dict(resp.headers)
                    )
                except (httpx.TimeoutException, httpx.RequestError) as e:
                    if attempt == max_retries:
                        # Si fallaron todos los intentos, arrojamos el error final
                        raise e
                    # Breve pausa antes del siguiente reintento
                    await asyncio.sleep(1)
        except Exception as e:
            # Esto será capturado por el manejador global en main.py (ej. 502 Bad Gateway)
            raise e

    async def close(self):
        """Cierra el cliente HTTP."""
        await self.client.aclose()

proxy_service = ProxyService()

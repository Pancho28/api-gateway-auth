from fastapi import APIRouter, Request, Depends
from services.proxy_service import proxy_service
from api.dependencies import get_current_user
from models.user import User
from core.logger import get_logger
from core.rate_limit import limiter

logger = get_logger(__name__)
router = APIRouter(tags=["Gateway"])

@router.get("/health")
@limiter.limit("100/minute")
async def health_check(request: Request):
    """Endpoint público de salud del Gateway."""
    logger.info("Verificación de salud (health check) solicitada.")
    return {
        "status": "healthy",
        "gateway_version": "1.0.0",
        "system": "API Gateway FastAPI"
    }

@router.api_route("/{path:path}", methods=["GET", "POST", "PUT", "DELETE", "PATCH"])
@limiter.limit("50/minute")
async def reverse_proxy(
    request: Request, 
    path: str, 
    current_user: User = Depends(get_current_user),
    service=Depends(lambda: proxy_service)
):
    """
    Ruta interceptora global que redirige el tráfico.
    """
    logger.info(f"Interceptando petición hacia: /{path} (Método: {request.method}) solicitada por el usuario: {current_user.email}")
    result = await service.forward_request(request, path)
    logger.info(f"Petición hacia /{path} completada.")
    return result

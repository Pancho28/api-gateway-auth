from fastapi import APIRouter, Depends, Request
from api.schemas.auth import Token
from api.schemas.user import UserLogin
from services.auth_service import auth_service
from sqlalchemy.ext.asyncio import AsyncSession
from core.database import get_db
from core.logger import get_logger
from core.rate_limit import limiter

logger = get_logger(__name__)
router = APIRouter(prefix="/auth", tags=["Auth"])

@router.post("/login", response_model=Token)
@limiter.limit("10/minute")
async def login(
    request: Request,
    user_in: UserLogin, 
    db: AsyncSession = Depends(get_db),
    service=Depends(lambda: auth_service)
):
    """Endpoint para el inicio de sesión."""
    logger.info(f"Intento de inicio de sesión para el correo: {user_in.email}")
    result = await service.login_user(db, user_in)
    logger.info(f"Inicio de sesión exitoso para el correo: {user_in.email}")
    return result

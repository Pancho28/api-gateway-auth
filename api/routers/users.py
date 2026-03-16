from fastapi import APIRouter, Depends, status, Request
from sqlalchemy.ext.asyncio import AsyncSession

from api.schemas.user import AdminUserCreate, UserResponse
from services.user_service import user_service
from core.database import get_db
from api.dependencies import get_current_admin_user
from models.user import User
from core.logger import get_logger
from core.rate_limit import limiter

logger = get_logger(__name__)
router = APIRouter(prefix="/users", tags=["Users"])

@router.post("", response_model=UserResponse, status_code=status.HTTP_201_CREATED)
@limiter.limit("50/minute")
async def create_user(
    request: Request,
    user_in: AdminUserCreate,
    current_admin: User = Depends(get_current_admin_user),
    db: AsyncSession = Depends(get_db)
):
    """
    Endpoint para que un administrador cree nuevos usuarios en el sistema.
    Solo accesible por usuarios con rol ADMIN.
    """
    logger.info(f"Administrador {current_admin.email} intenta crear un usuario nuevo: {user_in.email}")
    new_user = await user_service.create_user_by_admin(db, user_in)
    logger.info(f"Usuario {new_user.email} (Rol: {new_user.role}) creado exitosamente por administrador {current_admin.email}")
    return new_user

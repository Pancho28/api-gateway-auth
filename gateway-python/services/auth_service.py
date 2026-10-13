from fastapi import HTTPException, status
from sqlalchemy.ext.asyncio import AsyncSession
from sqlalchemy import select
from core.security import verify_password, create_access_token
from api.schemas.user import UserLogin
from models.user import User
from core.logger import get_logger
from core.redis import set_cached_json
from core.config import settings

logger = get_logger(__name__)

class AuthService:
    """Lógica de negocio para la autenticación."""
    
    async def login_user(self, db: AsyncSession, user_in: UserLogin):
        """Lógica para autenticar a un usuario validando credenciales en BD y cacheando sesión en Redis."""
        stmt = select(User).where(User.email == user_in.email)
        result = await db.execute(stmt)
        user = result.scalars().first()
        
        if not user or not verify_password(user_in.password, user.hashed_password):
            logger.warning(f"Intento de inicio de sesión fallido para: {user_in.email}")
            raise HTTPException(
                status_code=status.HTTP_401_UNAUTHORIZED,
                detail="Correo o contraseña incorrectos",
                headers={"WWW-Authenticate": "Bearer"},
            )
        
        access_token = create_access_token(data={"sub": user.email, "role": user.role})

        # Almacenar sesión en Redis con TTL igual a la expiración del JWT (ADR-002)
        await set_cached_json(
            f"session:{user.email}",
            {"id": user.id, "email": user.email, "role": user.role},
            ttl=settings.ACCESS_TOKEN_EXPIRE_MINUTES * 60
        )

        return {"user": user.email, "role": user.role, "token": access_token}

auth_service = AuthService()

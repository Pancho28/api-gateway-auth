from fastapi import HTTPException, status
from sqlalchemy.ext.asyncio import AsyncSession
from sqlalchemy import select
from core.security import verify_password, create_access_token
from api.schemas.user import UserLogin
from models.user import User
from core.logger import get_logger

logger = get_logger(__name__)

class AuthService:
    """Lógica de negocio para la autenticación."""
    
    async def login_user(self, db: AsyncSession, user_in: UserLogin):
        """Lógica para autenticar a un usuario validando credenciales en BD."""
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
        return {"user": user.email, "role": user.role, "token": access_token}

auth_service = AuthService()

from fastapi import HTTPException, status
from sqlalchemy.ext.asyncio import AsyncSession
from sqlalchemy import select
from sqlalchemy.exc import IntegrityError

from core.security import get_password_hash
from api.schemas.user import AdminUserCreate
from models.user import User
from core.logger import get_logger

logger = get_logger(__name__)

class UserService:
    """Lógica de negocio para la gestión de usuarios."""
    
    async def create_user_by_admin(self, db: AsyncSession, user_in: AdminUserCreate) -> User:
        """Lógica para crear un usuario, verificando si ya existe."""
        
        # Verificar si el correo ya existe
        stmt = select(User).where(User.email == user_in.email)
        result = await db.execute(stmt)
        if result.scalars().first():
            logger.warning(f"Intento de crear usuario con correo ya registrado: {user_in.email}")
            raise HTTPException(
                status_code=status.HTTP_400_BAD_REQUEST,
                detail="El correo electrónico ya está registrado."
            )
            
        hashed_pass = get_password_hash(user_in.password)
        
        new_user = User(
            email=user_in.email,
            hashed_password=hashed_pass,
            role=user_in.role.upper()
        )
        
        db.add(new_user)
        try:
            await db.commit()
            await db.refresh(new_user)
            logger.info(f"Nuevo usuario creado exitosamente: {new_user.email}")
            return new_user
        except IntegrityError as e:
            await db.rollback()
            logger.error(f"Error de integridad al crear usuario {user_in.email}: {str(e)}")
            raise HTTPException(
                status_code=status.HTTP_400_BAD_REQUEST,
                detail="Error en la base de datos al registrar el usuario."
            )
        except Exception as e:
            await db.rollback()
            logger.critical(f"Error interno al crear usuario {user_in.email}: {str(e)}")
            raise HTTPException(
                status_code=status.HTTP_500_INTERNAL_SERVER_ERROR,
                detail="Error interno del servidor al crear el usuario."
            )

user_service = UserService()

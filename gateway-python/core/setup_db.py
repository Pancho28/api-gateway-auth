from sqlalchemy.ext.asyncio import AsyncSession
from sqlalchemy import select
from models.user import User
from core.config import settings
from core.security import get_password_hash
from core.logger import get_logger

logger = get_logger(__name__)

async def initialize_admin(session: AsyncSession):
    """
    Verifica si existe un usuario administrador. 
    Si no existe, lo crea usando las variables de entorno.
    """
    # Buscar si ya existe el usuario con el rol y correo del admin
    stmt = select(User).where(User.email == settings.EMAIL_ADMIN)
    result = await session.execute(stmt)
    admin_user = result.scalars().first()

    if admin_user:
        logger.info(f"El administrador ya existe.")
        return

    # Si no existe, crear el usuario administrador
    admin_password_hashed = get_password_hash(settings.PASSWORD_ADMIN)
    new_admin = User(
        email=settings.EMAIL_ADMIN,
        hashed_password=admin_password_hashed,
        role="ADMIN"
    )
    
    session.add(new_admin)
    try:
        await session.commit()
        logger.info(f"Se ha creado el usuario administrador.")
    except Exception as e:
        await session.rollback()
        logger.error(f"Error al crear el usuario administrador: {str(e)}")

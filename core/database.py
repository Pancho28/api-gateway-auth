from sqlalchemy.ext.asyncio import create_async_engine, async_sessionmaker, AsyncSession
from sqlalchemy.orm import DeclarativeBase
from core.config import settings

# Configuración de create_async_engine para MySQL usando aiomysql
# Se implementa una gestión de pool estricta para asegurar estabilidad
engine = create_async_engine(
    settings.ASYNC_DATABASE_URL,
    echo=False,
    pool_size=10,           # Capacidad base del pool
    max_overflow=20,        # Conexiones extra permitidas sobre el pool_size
    pool_recycle=3600,      # Reciclar conexiones cada hora para evitar timeouts de MySQL
    pool_pre_ping=True,     # Verificar salud de la conexión antes de cada uso
)

# Definición de la clase Base usando DeclarativeBase de SQLAlchemy 2.0
class Base(DeclarativeBase):
    pass

# Factoría de sesiones asíncronas
async_session_factory = async_sessionmaker(
    bind=engine,
    autoflush=False,
    autocommit=False,
    expire_on_commit=False,
    class_=AsyncSession
)

async def get_db():
    """
    Función generadora para inyección de dependencias en FastAPI.
    Asegura que la sesión se cierre correctamente tras su uso.
    """
    async with async_session_factory() as session:
        try:
            yield session
        finally:
            await session.close()

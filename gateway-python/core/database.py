from sqlalchemy.ext.asyncio import create_async_engine, async_sessionmaker, AsyncSession
from sqlalchemy.orm import DeclarativeBase
from core.config import settings

# Argumentos de conexión específicos para asyncpg y compatibilidad con PgBouncer en transaction mode (ADR-001)
ASYNC_CONNECT_ARGS = {
    "statement_cache_size": 0,
    "prepared_statement_cache_size": 0,
}

# Configuración de create_async_engine para PostgreSQL usando asyncpg
# Compatible tanto para conexión directa como mediante PgBouncer en transaction mode
engine = create_async_engine(
    settings.ASYNC_DATABASE_URL,
    echo=False,
    pool_size=10,           # Capacidad base del pool local
    max_overflow=20,        # Conexiones extra permitidas
    pool_recycle=1800,      # Reciclar conexiones periódicamente
    pool_pre_ping=True,     # Verificar salud de la conexión antes de cada uso
    connect_args=ASYNC_CONNECT_ARGS,
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

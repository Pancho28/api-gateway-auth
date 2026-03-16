import asyncio
import sys
import os

# Añadir el directorio raíz al path para poder importar core y modelos
sys.path.append(os.path.dirname(os.path.dirname(os.path.abspath(__file__))))

from core.database import engine, Base
from models.user import User # Importar para que Metadata las reconozca
from core.logger import get_logger

logger = get_logger(__name__)

async def reset_database():
    """
    Elimina todas las tablas y las recrea.
    Útil para cambios estructurales en desarrollo (como cambiar un ID a UUID).
    """
    logger.info("Iniciando reset de la base de datos...")
    try:
        async with engine.begin() as conn:
            # Eliminar todas las tablas registradas en Base.metadata
            logger.info("Eliminando tablas existentes...")
            await conn.run_sync(Base.metadata.drop_all)
            
            # Crear todas las tablas
            logger.info("Creando nuevas tablas...")
            await conn.run_sync(Base.metadata.create_all)
            
        logger.info("¡Base de datos reseteada con éxito!")
    except Exception as e:
        logger.error(f"Error durante el reset de la base de datos: {str(e)}")
    finally:
        await engine.dispose()

if __name__ == "__main__":
    if sys.platform == "win32":
        asyncio.set_event_loop_policy(asyncio.WindowsSelectorEventLoopPolicy())
    asyncio.run(reset_database())

import asyncio
import sys
import os

# Añadir el directorio raíz al path para poder importar core
sys.path.append(os.path.dirname(os.path.dirname(os.path.abspath(__file__))))

from sqlalchemy import text
from core.database import engine
from core.logger import get_logger

logger = get_logger(__name__)

async def verify_tables():
    """
    Verifica que la tabla 'users' existe en la base de datos.
    """
    try:
        async with engine.connect() as conn:
            # Intentar ver la descripción de la tabla
            result = await conn.execute(text("DESCRIBE users"))
            columns = result.all()
            if columns:
                logger.info("La tabla 'users' existe con las siguientes columnas:")
                for col in columns:
                    logger.info(f"- {col[0]}: {col[1]}")
            else:
                logger.warning("Error: La consulta 'DESCRIBE users' no devolvió resultados.")
    except Exception as e:
        logger.error(f"Error al verificar la tabla: {str(e)}")
    finally:
        await engine.dispose()

if __name__ == "__main__":
    if sys.platform == "win32":
        asyncio.set_event_loop_policy(asyncio.WindowsSelectorEventLoopPolicy())
    asyncio.run(verify_tables())

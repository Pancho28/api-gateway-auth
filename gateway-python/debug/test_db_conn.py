import asyncio
import sys
import os

# Añadir el directorio raíz al path para poder importar core
sys.path.append(os.path.dirname(os.path.dirname(os.path.abspath(__file__))))

from sqlalchemy import text
from core.database import engine
from core.logger import get_logger

logger = get_logger(__name__)

async def test_connection():
    """
    Intenta conectar a la base de datos y realizar una consulta simple (SELECT 1).
    Imprime el resultado en español.
    """
    try:
        async with engine.connect() as conn:
            # Intentar ejecutar una consulta simple
            result = await conn.execute(text("SELECT 1"))
            val = result.scalar()
            if val == 1:
                logger.info("Conexión exitosa")
            else:
                logger.warning(f"La consulta devolvió un valor inesperado: {val}")
    except Exception as e:
        logger.error(f"Error específico al conectar: {str(e)}")
    finally:
        # Cerrar el engine para liberar recursos detectados por asyncpg
        await engine.dispose()

if __name__ == "__main__":
    if sys.platform == "win32":
        asyncio.set_event_loop_policy(asyncio.WindowsSelectorEventLoopPolicy())
    asyncio.run(test_connection())

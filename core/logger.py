import logging
import sys

def get_logger(name: str) -> logging.Logger:
    """
    Retorna un logger estandarizado que se integra con el formato de Uvicorn.
    Debe usarse en todo el proyecto en lugar de 'print'.
    """
    # Intentar obtener el logger de uvicorn si existe
    logger = logging.getLogger(name)
    
    # Si el logger no tiene handlers (por ejemplo, si no corre bajo uvicorn), 
    # le añadimos un StreamHandler básico para que igual funcione en scripts sueltos
    if not logger.handlers:
        handler = logging.StreamHandler(sys.stdout)
        # Formato similar al de uvicorn (Nivel: Mensaje)
        formatter = logging.Formatter('%(levelname)s:\t  %(message)s')
        handler.setFormatter(formatter)
        logger.addHandler(handler)
        
    logger.setLevel(logging.INFO) # Nivel por defecto
    return logger

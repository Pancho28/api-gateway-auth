from contextlib import asynccontextmanager
import uuid
import httpx
from fastapi import FastAPI, Request, status
from fastapi.responses import JSONResponse

from api.routers import auth, gateway, users
from services.proxy_service import proxy_service
from core.database import engine, Base, async_session_factory
# Importar modelos para asegurar que se registren en Base.metadata
from models.user import User
from core.setup_db import initialize_admin
from core.logger import get_logger
from core.config import settings
from core.rate_limit import limiter
from slowapi import _rate_limit_exceeded_handler
from slowapi.errors import RateLimitExceeded
from fastapi.middleware.cors import CORSMiddleware

logger = get_logger(__name__)

@asynccontextmanager
async def lifespan(app: FastAPI):
    """
    Gestiona el ciclo de vida de la aplicación:
    - Crea las tablas al iniciar.
    - Inicializa el admin por defecto si es necesario.
    - Cierra recursos de red al apagar.
    """
    # Crear tablas si no existen
    logger.info("Iniciando creación de tablas (si no existen)...")
    async with engine.begin() as conn:
        await conn.run_sync(Base.metadata.create_all)
    logger.info("Verificación de tablas completada.")
    
    # Inicializar administrador
    logger.info("Iniciando proceso de seeding...")
    async with async_session_factory() as session:
        await initialize_admin(session)
    logger.info("Proceso de seeding finalizado.")
    
    logger.info("API Gateway iniciado correctamente.")
    yield
    
    # Cerrar recursos al apagar
    logger.info("Apagando API Gateway, cerrando conexiones proxy...")
    await proxy_service.close()
    logger.info("Cerrando motor de base de datos (engine)...")
    await engine.dispose()
    logger.info("Cerrando conexiones de Redis...")
    from core.redis import close_redis
    await close_redis()
    logger.info("Recursos liberados. Apagado exitoso.")

app = FastAPI(
    title="API Gateway", 
    version="1.0.0",
    lifespan=lifespan
)

# --- Configuración Rate Limit ---
app.state.limiter = limiter
app.add_exception_handler(RateLimitExceeded, _rate_limit_exceeded_handler)

# --- Configuración CORS ---
if settings.ORIGIN:
    logger.info(f"Configurando CORS para el origen: {settings.ORIGIN}")
    allow_origins = [settings.ORIGIN]
else:
    logger.info("El origen CORS está vacío. Configurando CORS para permitir cualquier origen (*)")
    allow_origins = ["*"]

app.add_middleware(
    CORSMiddleware,
    allow_origins=allow_origins,
    allow_credentials=True,
    allow_methods=["*"],
    allow_headers=["*"],
)

# --- Manejadores Globales de Excepciones ---

@app.exception_handler(Exception)
async def global_exception_handler(request: Request, exc: Exception):
    """
    Captura cualquier error interno no controlado (500).
    Esteriliza los logs registrando un ErrorID y el tipo de excepción,
    evitando la exposición involuntaria de credenciales o cadenas de conexión.
    """
    error_id = str(uuid.uuid4())[:12]
    logger.critical(
        f"Error interno no controlado [ErrorID: {error_id}] en {request.method} {request.url.path} (Tipo: {exc.__class__.__name__})"
    )
    return JSONResponse(
        status_code=status.HTTP_500_INTERNAL_SERVER_ERROR,
        content={"detail": "Error interno del servidor"},
        headers={"X-Error-ID": error_id}
    )

@app.exception_handler(httpx.HTTPStatusError)
@app.exception_handler(httpx.RequestError)
async def gateway_error_handler(request: Request, exc: Exception):
    """
    Captura errores de comunicación con microservicios externos (502).
    Esteriliza los logs evitando filtrar URLs internas o parámetros confidenciales.
    """
    error_id = str(uuid.uuid4())[:12]
    logger.error(
        f"Error de Gateway [ErrorID: {error_id}] al comunicar con microservicio desde {request.method} {request.url.path} (Tipo: {exc.__class__.__name__})"
    )
    return JSONResponse(
        status_code=status.HTTP_502_BAD_GATEWAY,
        content={"detail": "Bad Gateway: Error al comunicar con el servicio de destino"},
        headers={"X-Error-ID": error_id}
    )

# --- Registro de Routers ---

# El router de Auth se registra primero
app.include_router(auth.router)
app.include_router(users.router)

# El router de Gateway (catch-all) se registra AL FINAL
app.include_router(gateway.router)

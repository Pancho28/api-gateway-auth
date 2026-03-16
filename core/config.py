import json
from pydantic import field_validator
from pydantic_settings import BaseSettings, SettingsConfigDict

class Settings(BaseSettings):
    """
    Configuración global de la aplicación utilizando pydantic-settings.
    Carga las variables desde el archivo .env si existe.
    """
    SECRET_KEY: str
    ALGORITHM: str = "HS256"
    ACCESS_TOKEN_EXPIRE_MINUTES: int = 30
    
    # Configuración de Base de Datos (MySQL)
    DB_USER: str
    DB_PASSWORD: str
    DB_HOST: str = "localhost"
    DB_PORT: int = 3306
    DB_NAME: str

    # Credenciales del Administrador por defecto
    EMAIL_ADMIN: str
    PASSWORD_ADMIN: str

    # Origen CORS permitido, si vacío, permite todos
    ORIGIN: str = ""

    # Configuración unificada de microservicios: {host: {url, api_key}}
    MICROSERVICES: dict[str, dict[str, str]]

    @field_validator("MICROSERVICES", mode="before")
    @classmethod
    def parse_json_strings(cls, v: str | dict) -> dict:
        """Parsea los strings JSON a diccionarios de Python."""
        if isinstance(v, str):
            try:
                return json.loads(v)
            except json.JSONDecodeError:
                raise ValueError("El valor de MICROSERVICES debe ser un JSON string válido")
        return v

    @property
    def ASYNC_DATABASE_URL(self) -> str:
        """Genera la URL de conexión asíncrona para SQLAlchemy."""
        return f"mysql+aiomysql://{self.DB_USER}:{self.DB_PASSWORD}@{self.DB_HOST}:{self.DB_PORT}/{self.DB_NAME}"

    model_config = SettingsConfigDict(env_file=".env", extra="ignore")

settings = Settings()

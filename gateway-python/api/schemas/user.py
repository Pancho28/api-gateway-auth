from typing import Literal
from pydantic import BaseModel, EmailStr, Field, field_validator

class UserLogin(BaseModel):
    """Esquema para el inicio de sesión con límites de seguridad contra DoS."""
    email: EmailStr = Field(..., max_length=50, description="Correo electrónico del usuario")
    password: str = Field(..., max_length=40, description="Contraseña del usuario")

class AdminUserCreate(BaseModel):
    """
    Esquema para la creación de un nuevo usuario por parte de un administrador.
    Valida complejidad de clave, longitudes máximas y roles estrictos.
    """
    email: EmailStr = Field(..., max_length=50, description="Correo electrónico del nuevo usuario")
    password: str = Field(
        ...,
        min_length=8,
        max_length=40,
        description="Contraseña del nuevo usuario (8-40 caracteres, alfanumérica)"
    )
    role: Literal["ADMIN", "USER"] = Field(..., description="Rol del nuevo usuario (ADMIN, USER)")

    @field_validator("password")
    @classmethod
    def validate_password_complexity(cls, v: str) -> str:
        """Valida que la contraseña contenga al menos una letra y un número."""
        if not (any(c.isalpha() for c in v) and any(c.isdigit() for c in v)):
            raise ValueError("La contraseña debe contener al menos una letra y un número")
        return v

    @field_validator("role", mode="before")
    @classmethod
    def normalize_role(cls, v: str) -> str:
        """Normaliza el rol a mayúsculas para asegurar consistencia."""
        if isinstance(v, str):
            return v.upper()
        return v

class UserResponse(BaseModel):
    """Esquema de respuesta al crear o consultar un usuario."""
    id: str
    email: EmailStr = Field(..., max_length=50)
    role: Literal["ADMIN", "USER"]

    class Config:
        from_attributes = True

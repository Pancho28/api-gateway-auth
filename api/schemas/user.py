from pydantic import BaseModel, EmailStr, Field

class UserLogin(BaseModel):
    """Esquema para el inicio de sesión."""
    email: EmailStr
    password: str

class AdminUserCreate(BaseModel):
    """
    Esquema para la creación de un nuevo usuario por parte de un administrador.
    Valida que email, clave y rol no sean nulos.
    """
    email: EmailStr = Field(..., description="Correo electrónico del nuevo usuario")
    password: str = Field(..., min_length=6, description="Contraseña del nuevo usuario")
    role: str = Field(..., description="Rol del nuevo usuario (ej. ADMIN, USER)")

class UserResponse(BaseModel):
    """Esquema de respuesta al crear un usuario."""
    id: str
    email: EmailStr
    role: str

    class Config:
        from_attributes = True

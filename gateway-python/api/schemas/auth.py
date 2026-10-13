from pydantic import BaseModel, EmailStr

class Token(BaseModel):
    """Esquema para el retorno del token de acceso."""
    user: str
    role: str
    token: str


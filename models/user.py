import uuid
from sqlalchemy.orm import Mapped, mapped_column
from sqlalchemy import String, CHAR
from core.database import Base

class User(Base):
    """
    Modelo de Usuario para el esquema de autenticación.
    Utiliza UUID como clave primaria para mejor seguridad y escalabilidad.
    """
    __tablename__ = "users"

    id: Mapped[str] = mapped_column(
        CHAR(36), 
        primary_key=True, 
        default=lambda: str(uuid.uuid4()),
        index=True
    )
    email: Mapped[str] = mapped_column(String(255), unique=True, nullable=False, index=True)
    hashed_password: Mapped[str] = mapped_column(String(255), nullable=False)
    role: Mapped[str] = mapped_column(String(50), nullable=False, default="user")

    def __repr__(self) -> str:
        return f"<User(id={self.id}, email={self.email}, role={self.role})>"

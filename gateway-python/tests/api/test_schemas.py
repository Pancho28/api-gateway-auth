import pytest
from pydantic import ValidationError
from api.schemas.user import AdminUserCreate, UserLogin, UserResponse

def test_admin_user_create_valid():
    """Valida que un usuario con datos válidos cumpla con el esquema."""
    user = AdminUserCreate(email="valid@example.com", password="SecurePassword1", role="ADMIN")
    assert user.email == "valid@example.com"
    assert user.role == "ADMIN"

def test_admin_user_create_password_too_short():
    """Valida que contraseñas menores a 8 caracteres sean rechazadas."""
    with pytest.raises(ValidationError) as exc:
        AdminUserCreate(email="valid@example.com", password="short1", role="USER")
    assert "password" in str(exc.value)

def test_admin_user_create_password_too_long():
    """Valida que contraseñas mayores a 40 caracteres sean rechazadas."""
    long_pass = "A1" + "a" * 39  # 41 caracteres
    with pytest.raises(ValidationError) as exc:
        AdminUserCreate(email="valid@example.com", password=long_pass, role="USER")
    assert "password" in str(exc.value)

def test_admin_user_create_password_regex_complexity():
    """Valida que la contraseña deba contener al menos letras y números."""
    # Solo letras
    with pytest.raises(ValidationError) as exc:
        AdminUserCreate(email="valid@example.com", password="OnlyLettersHere", role="USER")
    assert "password" in str(exc.value)

    # Solo números
    with pytest.raises(ValidationError) as exc:
        AdminUserCreate(email="valid@example.com", password="1234567890", role="USER")
    assert "password" in str(exc.value)

def test_admin_user_create_email_too_long():
    """Valida que un correo mayor a 50 caracteres sea rechazado."""
    long_email = "a" * 40 + "@example.com"  # 52 caracteres
    with pytest.raises(ValidationError) as exc:
        AdminUserCreate(email=long_email, password="ValidPassword1", role="USER")
    assert "email" in str(exc.value)

def test_admin_user_create_invalid_role():
    """Valida que roles no permitidos sean rechazados."""
    with pytest.raises(ValidationError) as exc:
        AdminUserCreate(email="valid@example.com", password="ValidPassword1", role="SUPERADMIN")
    assert "role" in str(exc.value)

def test_user_login_limits():
    """Valida restricciones de tamaño en UserLogin para mitigar DoS."""
    # Email > 50 chars
    long_email = "b" * 40 + "@example.com"
    with pytest.raises(ValidationError) as exc:
        UserLogin(email=long_email, password="ValidPassword1")
    assert "email" in str(exc.value)

    # Password > 40 chars
    long_pass = "P" * 41
    with pytest.raises(ValidationError) as exc:
        UserLogin(email="valid@example.com", password=long_pass)
    assert "password" in str(exc.value)

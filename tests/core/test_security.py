from core.security import verify_password, get_password_hash, create_access_token, decode_access_token

def test_password_hashing():
    """Valida la generación y verificación de hashes de contraseñas."""
    pwd = "my_secure_password"
    hashed = get_password_hash(pwd)
    
    assert hashed != pwd
    assert verify_password(pwd, hashed) is True
    assert verify_password("wrong_password", hashed) is False

def test_jwt_token_lifecycle():
    """Valida la creación y decodificación exitosa de un token JWT."""
    data = {"sub": "admin@test.com", "role": "ADMIN"}
    
    # Crear token
    token = create_access_token(data)
    assert isinstance(token, str)
    
    # Decodificar token válido
    decoded = decode_access_token(token)
    assert decoded is not None
    assert decoded["sub"] == "admin@test.com"
    assert decoded["role"] == "ADMIN"
    assert "exp" in decoded

def test_decode_invalid_token():
    """Valida que la decodificación de un token inválido retorne None."""
    invalid_token = "ey.invalid.signature"
    assert decode_access_token(invalid_token) is None

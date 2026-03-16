import asyncio
import httpx
import time
import os
from dotenv import load_dotenv

# Cargar variables de entorno (para sacar usuario/pwd y microservicios si fuera necesario)
load_dotenv()

# Configuración Base
BASE_URL = "http://127.0.0.1:8000"
LOGIN_URL = f"{BASE_URL}/auth/login"
PROXY_URL = f"{BASE_URL}/items/42" # Ruta ficticia en el proxy intermedio

# Credenciales desde el .env
EMAIL = os.getenv("EMAIL_ADMIN", "admin@gmail.com")
PASSWORD = os.getenv("PASSWORD_ADMIN", "123456")

# Host configurado en MICROSERVICES (para el encabezado HOST que activa el enrutamiento del gateway)
TARGET_HOST = "ventas.local"

# Configuración de Estrés
NUM_REQUESTS = 75   # El límite para endpoints privados logueados es 50/usuario/minuto
CONCURRENCY = 10    

async def login(client):
    """Realiza el login y retorna el token JWT."""
    print(f"🔐 Intentando login con {EMAIL}...")
    try:
        response = await client.post(LOGIN_URL, json={"email": EMAIL, "password": PASSWORD})
        response.raise_for_status()
        data = response.json()
        print(f"✅ Login exitoso. Token obtenido.\n")
        return data.get("token")
    except Exception as e:
        print(f"❌ Error crítico en Login: {str(e)}")
        if hasattr(e, 'response') and e.response:
            print(f"Detalle: {e.response.text}")
        return None

async def make_proxy_request(client, token, req_id):
    """Ejecuta una petición al gateway enviando el Token Bearer y el header Host."""
    headers = {
        "Authorization": f"Bearer {token}",
        "Host": TARGET_HOST
    }
    try:
        response = await client.get(PROXY_URL, headers=headers)
        return response.status_code
    except Exception as e:
        return f"Error: {str(e)}"

async def main():
    print(f"🚀 Iniciando prueba de estrés: {NUM_REQUESTS} peticiones autenticadas al Gateway proxy...")
    print(f"⚠️  Asegúrate de tener corriendo: ./manage.sh gateway y ./manage.sh mock ventas\n")
    
    start_time = time.time()
    
    async with httpx.AsyncClient(limits=httpx.Limits(max_connections=CONCURRENCY)) as client:
        # 1. Obtener Token
        token = await login(client)
        if not token:
            print("Abortando prueba por fallo en autenticación.")
            return

        # 2. Atacar el endpoint proxy
        tasks = [make_proxy_request(client, token, i) for i in range(NUM_REQUESTS)]
        results = await asyncio.gather(*tasks)
    
    end_time = time.time()
    
    # Procesar resultados
    success_200 = results.count(200)
    unauthorized_401 = results.count(401)
    not_found_404 = results.count(404)
    rate_limited_429 = results.count(429)
    errors = len([r for r in results if isinstance(r, str)])
    others = len(results) - success_200 - unauthorized_401 - not_found_404 - rate_limited_429 - errors
    
    # Imprimir reporte
    print("📊 --- REPORTE DE ESTRÉS (USUARIO AUTENTICADO) ---")
    print(f"⏱️  Tiempo total: {end_time - start_time:.2f} segundos")
    print(f"✅ Exitosas (200 OK): {success_200}")
    print(f"🛑 Bloqueadas por Rate Limit (429 Too Many Requests): {rate_limited_429}")
    
    if unauthorized_401 > 0:
        print(f"❌ Problemas de Auth (401 Unauthorized): {unauthorized_401}")
    if not_found_404 > 0:
        print(f"⚠️  No encontrado (404 Not Found): {not_found_404} -> (Mocks apagados o Host mapping inválido)")
    if errors > 0:
        print(f"⚠️ Errores de red/conexión al Gateway: {errors}")
    if others > 0:
        print(f"❓ Otros códigos de estado devueltos: {others}")
        
    print("\n💡 Análisis:")
    if rate_limited_429 > 0:
        print(f"✅ ¡Rate Limiter Híbrido funcionando! Bloqueó exactamente a 1 Usuario. {success_200} completadas, {rate_limited_429} interceptadas.")
    else:
        print("❌ No se registraron bloqueos 429. ¿El límite es mayor a 75 o está inactivo?")

if __name__ == "__main__":
    asyncio.run(main())

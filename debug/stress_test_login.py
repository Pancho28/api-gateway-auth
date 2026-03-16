import asyncio
import httpx
import time

# Configuración de la prueba
URL = "http://127.0.0.1:8000/auth/login"
PAYLOAD = {"email": "admin@example.com", "password": "wrong_password"}
NUM_REQUESTS = 150  # El límite configurado es 100/minuto para IPs públicas
CONCURRENCY = 20    # Cuántas peticiones hacer al mismo tiempo

async def make_request(client, req_id):
    """Ejecuta una petición individual y retorna el código de estado."""
    try:
        response = await client.post(URL, json=PAYLOAD)
        return response.status_code
    except Exception as e:
        return f"Error: {str(e)}"

async def main():
    print(f"🚀 Iniciando prueba de estrés: {NUM_REQUESTS} peticiones al endpoint de Login...")
    print(f"⚠️  Asegúrate de que el API Gateway esté corriendo en {URL}\n")
    
    start_time = time.time()
    
    # httpx.AsyncClient maneja un pool de conexiones para hacer requests eficientes
    async with httpx.AsyncClient(limits=httpx.Limits(max_connections=CONCURRENCY)) as client:
        # Crear la lista de tareas
        tasks = [make_request(client, i) for i in range(NUM_REQUESTS)]
        
        # Ejecutar todas concurrentemente
        results = await asyncio.gather(*tasks)
    
    end_time = time.time()
    
    # Procesar resultados
    success_200 = results.count(200)
    unauthorized_401 = results.count(401)
    rate_limited_429 = results.count(429)
    errors = len([r for r in results if isinstance(r, str)])
    others = len(results) - success_200 - unauthorized_401 - rate_limited_429 - errors
    
    # Imprimir reporte
    print("📊 --- REPORTE DE ESTRÉS ---")
    print(f"⏱️  Tiempo total: {end_time - start_time:.2f} segundos")
    print(f"✅ Autorizadas (200 OK): {success_200}")
    print(f"❌ Denegadas por credenciales (401 Unauthorized): {unauthorized_401}")
    print(f"🛑 Bloqueadas por Rate Limit (429 Too Many Requests): {rate_limited_429}")
    if errors > 0:
        print(f"⚠️ Errores de red/conexión: {errors}")
    if others > 0:
        print(f"❓ Otros códigos de estado: {others}")
        
    print("\n💡 Análisis:")
    if rate_limited_429 > 0:
        print("¡El Rate Limiting está funcionando! Ha bloqueado peticiones con un código 429.")
    else:
        print("No se registraron bloqueos 429. ¿Está el Rate Limiter configurado correctamente a menos de 150?")

if __name__ == "__main__":
    asyncio.run(main())

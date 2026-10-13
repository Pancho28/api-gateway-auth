# ADR-002: Estrategia de Caché de Sesiones, Proxy Caching y Rate Limiting usando Redis

* **Estado:** Aceptado
* **Fecha:** Octubre 2026
* **Decisores:** Senior Backend Architecture Team
* **Contexto Técnico:** API Gateway & Auth Infrastructure

---

## 1. Contexto y Problema

A medida que el API Gateway incrementa su volumen de tráfico y se prepara para operar en entornos multi-instancia (múltiples pods en Kubernetes o réplicas en Docker):
1. **Sobrecarga de Base de Datos en Autenticación:** Cada petición interceptada hacia microservicios protegidos valida el JWT y ejecuta una consulta SQL `SELECT` contra PostgreSQL para verificar existencia y rol del usuario. Con 1,000 req/s, esto genera 1,000 queries/s innecesarias a la BD, incrementando la latencia en 5-15ms por petición.
2. **Límite de Tasa Local (Inconsistencia Horizontal):** `slowapi` opera con almacenamiento en memoria local. Al desplegar 3 instancias del Gateway, un atacante o cliente puede realizar 3 veces más peticiones que el límite establecido.
3. **Peticiones Idempotentes Redundantes:** Microservicios internos reciben tráfico recurrente de consultas idénticas (catálogos de productos, listas de stock, configuraciones) que no cambian segundo a segundo, desperdiciando CPU y ancho de banda interno.

---

## 2. Decisión

Se adopta **Redis 7+** como capa centralizada de aceleración de ultra-baja latencia y estado distribuido:

1. **Cliente y Conectividad Asíncrona:**
   - Utilizar la librería oficial **`redis`** (con su paquete `redis.asyncio`) para garantizar operaciones asíncronas sin bloquear el Event Loop de FastAPI.
2. **Caché de Sesiones JWT (JWT Caching):**
   - En el endpoint `/auth/login`, tras verificar las credenciales en PostgreSQL, se almacena en Redis la clave `session:{email}` conteniendo la metadata del usuario (`id`, `email`, `role`) en formato JSON.
   - El TTL se sincroniza con el tiempo de expiración del JWT (`ACCESS_TOKEN_EXPIRE_MINUTES * 60`).
   - La dependencia `get_current_user` valida el token y consulta primeramente Redis. Si hay **Cache Hit**, el usuario se reconstituye en menos de 1ms sin tocar PostgreSQL.
   - Si ocurre **Cache Miss** (ej. expiración o primer acceso), consulta PostgreSQL y repuebla Redis (patrón *Cache-Aside*).
3. **Caché de Respuestas en Proxy (Proxy Caching):**
   - Interceptar peticiones con método `GET` hacia los microservicios.
   - Generar claves deterministas mediante hashing SHA-256 de la ruta y query parameters: `proxy_cache:{service_prefix}:{hash}`.
   - Respuestas HTTP 200 se guardan con un TTL predeterminado (60s) e inyectan la cabecera `X-Cache: HIT` o `X-Cache: MISS`.
4. **Rate Limiting Distribuido:**
   - Configurar `slowapi.Limiter` con `storage_uri=settings.REDIS_URL`, unificando las cuotas entre todas las instancias del Gateway.
5. **Estrategia de Resiliencia (Graceful Degradation):**
   - Si Redis está inalcanzable temporalmente, el sistema captura la excepción, registra una advertencia y realiza un fallback directo a PostgreSQL o al microservicio sin interrumpir la operación del cliente.

---

## 3. Consecuencias

### Positivas
* **Latencia Ultrabaja:** Reducción del tiempo de resolución de autenticación de ~10ms a <1ms en el 99% de las peticiones.
* **Alivio de Carga en DB:** Disminución de más del 95% de las lecturas SQL en PostgreSQL.
* **Consistencia Global de Seguridad:** Rate limiting estricto y unificado entre todas las réplicas del Gateway.
* **Revocación Inmediata de Tokens:** Posibilidad de invalidar sesiones al instante simplemente borrando la clave `session:{email}` en Redis.

### Trade-offs / Mitigaciones
* **Dependencia de Infraestructura:** Se requiere alta disponibilidad de Redis en producción (ej. Redis Sentinel o Redis Cluster).
* **Consistencia Eventual en Proxy:** Los datos cacheados en GET pueden tener un desfase máximo equivalente al TTL (60 segundos por defecto).

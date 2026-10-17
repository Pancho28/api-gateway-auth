# 🏗️ Especificación Técnica: API Gateway & Auth (SPEC_API_GATEWAY_AUTH)

**Fecha de última actualización:** Octubre 2026
**Rol / Enfoque:** Senior Backend Architecture & API Hardening

---

## 1. Visión General (Overview)

El proyecto `api-gateway-auth` actúa como el **Punto de Entrada Único (Single Point of Entry)** y la principal barrera de seguridad para la infraestructura de microservicios. 

Sus responsabilidades fundamentales son:
1. **Interceptación del Tráfico:** Recibir todas las peticiones de los clientes (Frontend, Apps móviles).
2. **Gestión de Identidad (AuthN/AuthZ):** Emitir tokens JWT, validar sesiones y hacer cumplir el Control de Acceso Basado en Roles (RBAC).
3. **Enrutamiento Proxy:** Reenviar el tráfico autorizado de manera eficiente y asíncrona hacia los microservicios internos correspondientes, ocultando la topología de la red interna.

---

## 2. Arquitectura y Estado Actual (Current Architecture State)

### Stack Tecnológico
*   **Core:** Go 1.24+, Chi (`github.com/go-chi/chi/v5`).
*   **Base de Datos:** PostgreSQL 16 mediante `pgxpool` (`github.com/jackc/pgx/v5/pgxpool`) en Simple Protocol para PgBouncer.
*   **Caché & Estado:** Redis 7 mediante `go-redis/v9` y `redis_rate/v10`.
*   **Seguridad:** `golang-jwt/jwt/v5`, PBKDF2 (`golang.org/x/crypto/pbkdf2`), Zero-Trust Client Partitioning.
*   **Proxying:** `net/http/httputil.ReverseProxy` con streaming zero-copy nativo.

### Patrones de Diseño
*   **Layered Clean Architecture:** Separación entre capa de transporte (`internal/api`), lógica (`internal/auth`, `internal/user`, `internal/proxy`) e infraestructura (`internal/database`, `internal/redis`, `internal/security`).
*   **I/O Concurrente y Streaming Zero-Copy:** Uso de goroutines y streaming HTTP nativo con `io.LimitReader` para caché acotada en Redis (máx 1 MB).
*   **Zero-Trust Client Partitioning & Token Binding:** Enlace criptográfico entre `X-Client-Id` y claims del token con política restrictiva deny-by-default en microservicios.

### Flujo de Petición (Request Flow)
1. Petición entra al Gateway.
2. Interceptada por `Recoverer`, `CORS` y `RateLimiter` en Redis.
3. Si la ruta está protegida, `AuthMiddleware` valida el JWT y la sesión en **Redis** (<0.5 ms), sin tocar PostgreSQL en Cache Hits.
4. Llega al Handler Catch-All (`/*`).
5. `proxy.Service` valida particionamiento Zero-Trust (`allowed_clients`), purga cabeceras sensibles del cliente, inyecta `X-Internal-Api-Key` y las cabeceras fidedignas (`X-Client-ID`, `X-Scopes`, `X-User-*`), y reenvía al microservicio objetivo.

---

## 3. Postura de Seguridad (Security Posture)

### Fortalezas Actuales
*   Autenticación centralizada, evitando implementar validación de contraseñas en cada microservicio.
*   Enrutamiento por prefijo que mantiene las IPs/Puertos internos completamente ocultos del mundo exterior.
*   Protección contra ataques de fuerza bruta mediante limitación de tasa (10 req/min en login).

### Deuda Técnica de Seguridad (A Corregir en Próximas Iteraciones)
*   **Esquemas y Contraseñas:** Ausencia de validaciones de complejidad (Regex) y restricciones estrictas de Enum para los Roles en Pydantic (`api/schemas/user.py`).
*   **Header Spoofing:** El proxy actualmente copia las cabeceras del cliente con filtros insuficientes. Un atacante podría inyectar cabeceras internas (`X-Internal-Api-Key`) si no se sanean explícitamente antes del reenvío.
*   **Path Traversal:** El particionamiento de la URL (`sub_path`) en el proxy carece de validaciones de sanitización contra caracteres como `..` o `.`.
*   **Fugas de Información en Logs:** El manejador global de excepciones (Error 500) loguea la variable de la excepción en crudo, lo que podría exponer cadenas de conexión a base de datos.

---

## 4. Rendimiento y Cuellos de Botella Actuales (Performance & Limits)

*   **Cuello de botella en Validación de JWT:** Actualmente, por cada petición que pasa por el proxy protegido, la función `get_current_user` hace un `SELECT` a la base de datos MySQL para buscar al usuario (`await db.execute(stmt)`). Bajo alta concurrencia, esto generará mucha latencia y sobrecargará la base de datos de manera innecesaria.
*   **Rate Limiting Local:** `slowapi` se ejecuta en memoria. Si el API Gateway se escala horizontalmente (múltiples instancias/pods), el límite de peticiones dejará de ser global.
*   **Caché:** Todo el tráfico repetitivo genera procesamiento completo, consumiendo ancho de banda interno y tiempo de CPU en los microservicios.

---

## 5. Hoja de Ruta de Mejoras a Futuro (Roadmap)

Con el fin de evolucionar este Gateway hacia una arquitectura Enterprise robusta, escalable y altamente performante, se establece la siguiente hoja de ruta basada en las prioridades del proyecto:

### Fase 1: Hardening de Seguridad Inmediato (Quick Wins)
*Nota de Metodología:* Todas las tareas de esta fase se desarrollarán rigurosamente bajo **TDD** (Fase Red -> Fase Green -> Fase Refactor).

*   [x] Reforzar los esquemas de Pydantic con Regex para contraseñas (mínimo 8, máximo 40 caracteres con letras y números), límite de email (máximo 50 caracteres), max-lengths de seguridad y `Literal["ADMIN", "USER"]` para roles.
*   [x] Modificar `ProxyService` para aplicar una purga estricta (stripping) de cabeceras de confianza antes de procesar la petición hacia la red interna.
*   [x] Sanitizar `sub_path` en el proxy para evitar vulnerabilidades de Path Traversal.
*   [x] Esterilizar los logs de excepciones globales (Error 500).

### Fase 2: Modernización de Almacenamiento (Migración a PostgreSQL y PgBouncer)
*   **Motivación:** Mayor flexibilidad, mejor soporte para tipos de datos JSON complejos, capacidades geoespaciales nativas, y concurrencia superior mediante MVCC frente a MySQL. Además, preparar la arquitectura para soportar múltiples aplicaciones conectadas simultáneamente.
*   **Gestión de Conexiones (PgBouncer):** Implementar PgBouncer como *Connection Pooler* delante de PostgreSQL. Esto permitirá que el API Gateway pueda escalar horizontalmente (a múltiples instancias o pods) sin agotar las conexiones del sistema operativo de la base de datos (evitando el *Connection Exhaustion*). PgBouncer mantendrá conexiones persistentes y multiplexará miles de peticiones de clientes de forma extremadamente rápida.
*   **Acciones Realizadas:** 
    *   [x] Documentar ADR formal: [`docs/adr/0001-migracion-postgresql-asyncpg-pgbouncer.md`](docs/adr/0001-migracion-postgresql-asyncpg-pgbouncer.md).
    *   [x] Instalar dependencias asíncronas: `asyncpg==0.32.0` en `requirements.txt`.
    *   [x] Modificar la cadena de conexión en `.env` y `.env.sample` (`postgresql+asyncpg://`).
    *   [x] Configurar mitigación de prepared statements en `core/database.py` (`statement_cache_size=0`) para compatibilidad con PgBouncer en transaction mode.
    *   [x] Adaptar modelo `User` en `models/user.py` para compatibilidad con tipos estándar PostgreSQL.
    *   [x] Proveer `docker-compose.yml` con servicios orquestados de PostgreSQL 16 y PgBouncer.

### Fase 3: Aceleración y Resiliencia con Redis (Ultra-Baja Latencia)
*   **Acciones Realizadas:**
    *   [x] Documentar ADR formal: [`docs/adr/0002-estrategia-cache-sesiones-rate-limiting-redis.md`](docs/adr/0002-estrategia-cache-sesiones-rate-limiting-redis.md).
    *   [x] Instalar dependencia `redis==8.1.0` en el entorno virtual y registrar en `requirements.txt`.
    *   [x] Añadir servicio `redis:7-alpine` a `docker-compose.yml` con healthcheck y volumen persistente.
    *   [x] Crear módulo `core/redis.py` con pool de conexiones asíncrono y degradación elegante (Graceful Degradation).
    *   [x] **Validación de Sesiones Ultrarrápida (JWT Caching):** Almacenar sesión en Redis tras login en `auth_service.py` y validar en `get_current_user` (<1ms) sin consultas SQL a PostgreSQL.
    *   [x] **Caché de Peticiones Repetitivas (Proxy Caching):** Interceptar peticiones `GET` en `ProxyService`, cacheando respuestas exitosas (HTTP 200) con inyección de cabeceras `X-Cache: HIT` / `X-Cache: MISS`.
    *   [x] **Rate Limiting Distribuido:** Centralizar cuotas en Redis para escalabilidad horizontal con fallback automático a memoria si Redis no está activo.

### Fase 4: Observabilidad y Circuit Breaker
*   **Tracing Distribuido:** Integración con OpenTelemetry para inyectar `Trace-IDs` en las cabeceras proxy, permitiendo seguir el recorrido de la petición en herramientas como Jaeger o Datadog.
*   **Circuit Breaker:** Si el microservicio interno falla repetidas veces, el Gateway debe "abrir el circuito" y dejar de enviarle tráfico temporalmente (para evitar saturarlo mientras se recupera) devolviendo una respuesta *Fall-back* o un error 503 rápido.

### Fase 5: Aislamiento y Particionamiento de Clientes (Zero-Trust en Go)
*   [x] **Requerimiento Estricto de `X-Client-Id`:** Obligatoriedad de identificar el frontend emisor en `/auth/login` (rechazando peticiones anónimas con `400 Bad Request`).
*   [x] **Token Binding:** Validación de correspondencia entre el `client_id` embebido en el JWT y la cabecera `X-Client-Id` de la petición HTTP (`401 Unauthorized` si discrepan).
*   [x] **Zero-Trust Deny-by-Default:** Todo microservicio en `MICROSERVICES` debe declarar `allowed_clients` explícitos; los servicios sin configuración deniegan acceso perimetral con `403 Forbidden`.
*   [x] **Propagación Segura & Anti-Spoofing:** Purgado de cabeceras de cliente no confiables e inyección fidedigna aguas abajo de `X-Client-ID`, `X-Scopes`, `X-User-ID`, `X-User-Email`, `X-User-Role` y `X-Internal-Api-Key`.


---

## 6. Decisiones Arquitectónicas (ADRs)

Según lo estipulado en `agent.md`, toda implementación correspondiente a la Fase 2 en adelante debe ir acompañada de un **Architecture Decision Record (ADR)** ubicado en `docs/adr/`.

*   **ADRs Implementados:**
    *   [ADR-001: Migración de MySQL a PostgreSQL con asyncpg y Connection Pooling mediante PgBouncer](docs/adr/0001-migracion-postgresql-asyncpg-pgbouncer.md).
    *   [ADR-002: Estrategia de Caché de Sesiones, Proxy Caching y Rate Limiting usando Redis](docs/adr/0002-estrategia-cache-sesiones-rate-limiting-redis.md).
*   *Próximos ADRs a crear:* 
    *   `ADR-003: Trazabilidad Distribuida con OpenTelemetry y Circuit Breaking.`

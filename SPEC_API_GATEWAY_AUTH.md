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
*   **Core:** Python 3.11+, FastAPI (Framework web asíncrono).
*   **Base de Datos:** MySQL / MariaDB mediante `SQLAlchemy` (ORM) y `aiomysql` (driver asíncrono).
*   **Seguridad:** `python-jose` (JWT), `passlib` con `pbkdf2_sha256` (Hashing), `slowapi` (Rate Limiting).
*   **Proxying:** `httpx` (Cliente HTTP asíncrono con soporte de streaming).

### Patrones de Diseño
*   **Router -> Service:** Separación clara entre la capa de red/transporte (`api/routers/`) y la lógica de negocio pura (`services/`).
*   **I/O Asíncrono Non-Blocking:** Uso estricto de `async/await` para evitar el bloqueo del Event Loop durante llamadas a base de datos o microservicios.
*   **Proxy de Streaming:** Uso de `resp.aiter_raw()` para canalizar respuestas desde los microservicios al cliente en fragmentos, evitando picos de consumo de memoria RAM en respuestas grandes.

### Flujo de Petición (Request Flow)
1. Petición entra al Gateway.
2. Interceptada por Middleware CORS y Rate Limiter (`slowapi`).
3. Si la ruta está protegida, pasa por `get_current_user` (Dependencia): Se valida el JWT y se hace un **Query a la Base de Datos** para verificar la existencia del usuario.
4. Llega al Router Catch-All (`/{path:path}`).
5. `ProxyService` limpia cabeceras, inyecta `X-Internal-Api-Key` y reenvía al microservicio objetivo.

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

---

## 6. Decisiones Arquitectónicas (ADRs)

Según lo estipulado en `agent.md`, toda implementación correspondiente a la Fase 2 en adelante debe ir acompañada de un **Architecture Decision Record (ADR)** ubicado en `docs/adr/`.

*   **ADRs Implementados:**
    *   [ADR-001: Migración de MySQL a PostgreSQL con asyncpg y Connection Pooling mediante PgBouncer](docs/adr/0001-migracion-postgresql-asyncpg-pgbouncer.md).
    *   [ADR-002: Estrategia de Caché de Sesiones, Proxy Caching y Rate Limiting usando Redis](docs/adr/0002-estrategia-cache-sesiones-rate-limiting-redis.md).
*   *Próximos ADRs a crear:* 
    *   `ADR-003: Trazabilidad Distribuida con OpenTelemetry y Circuit Breaking.`

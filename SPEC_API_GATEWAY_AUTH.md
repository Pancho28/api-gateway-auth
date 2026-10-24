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

> [!IMPORTANT]
> **Metodología Transversal Obligatoria: TDD (Test-Driven Development)**  
> Todas las fases y nuevas implementaciones del Gateway deben desarrollarse rigurosamente bajo el ciclo **TDD**:
> 1. **Fase Red:** Diseñar y escribir las pruebas automatizadas (`*_test.go` o `test/api_test.go`) que verifiquen el nuevo comportamiento antes de programar la solución.
> 2. **Fase Green:** Implementar el código atómico y mínimo necesario para que las pruebas pasen satisfactoriamente.
> 3. **Fase Refactor:** Optimizar el diseño, la memoria y el rendimiento manteniendo la suite 100% en verde (`go test -v ./...`).

### Fase 1: Hardening de Seguridad Inmediato (Quick Wins)

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
*   **Tracing Distribuido con Jaeger (OpenTelemetry OTLP):**
    *   **Backend de Telemetría Seleccionado:** **Jaeger All-in-One** (`jaegertracing/all-in-one:latest`), desplegado como servicio interno en el mismo proyecto de Railway sobre la red privada (*Mesh IPv6*), garantizando **$0 de coste por transferencia** y latencia sub-milisegundo (<0.5 ms).
    *   **Protocolo Estándar:** Receptor OTLP/HTTP en el puerto interno `4318` (`http://jaeger.railway.internal:4318`), asegurando total compatibilidad sin cambios de protocolo entre **Go (Gateway)**, **NestJS/Next.js (ReservasManager)** y **Python (Pipelines de ETLs)**.
    *   **Inyección y Propagación:** El Gateway actúa como emisor raíz de la transacción, inyectando cabeceras estándar W3C `traceparent` hacia los microservicios downstream para consolidar diagramas en cascada (*waterfall/flamegraph*).
    *   **Dashboard Visual:** Interfaz web de Jaeger expuesta de forma segura en el puerto `16686` para auditoría y visualización de trazas.
*   **Circuit Breaker (Disyuntor Perimetral):**
    *   Implementación de disyuntor automático ante fallos reiterados en microservicios internos (timeouts o errores 5xx consecutivos).
    *   Estados: *Closed* (operación normal), *Open* (corte de tráfico y respuesta inmediata `503 Service Unavailable [E-05]` para proteger el microservicio y evitar saturación de descriptores de sockets en el Gateway) y *Half-Open* (sondeo controlado de recuperación).

### Fase 5: Aislamiento y Particionamiento de Clientes (Zero-Trust en Go)
*   [x] **Requerimiento Estricto de `X-Client-Id`:** Obligatoriedad de identificar el frontend emisor en `/auth/login` (rechazando peticiones anónimas con `400 Bad Request`).
*   [x] **Token Binding:** Validación de correspondencia entre el `client_id` embebido en el JWT y la cabecera `X-Client-Id` de la petición HTTP (`401 Unauthorized` si discrepan).
*   [x] **Zero-Trust Deny-by-Default:** Todo microservicio en `MICROSERVICES` debe declarar `allowed_clients` explícitos; los servicios sin configuración deniegan acceso perimetral con `403 Forbidden`.
*   [x] **Propagación Segura & Anti-Spoofing:** Purgado de cabeceras de cliente no confiables e inyección fidedigna aguas abajo de `X-Client-ID`, `X-Scopes`, `X-User-ID`, `X-User-Email`, `X-User-Role` y `X-Internal-Api-Key`.

### Fase 6: Ciclo de Vida de Sesiones & Operabilidad Perimetral
*   [x] **Política Estricta de Sesión de 1 Hora (Sin Refresh Token):**
    *   Vida útil inmutable del token JWT fijada a **60 minutos** (`ACCESS_TOKEN_EXPIRE_MINUTES=60`).
    *   TTL de persistencia de sesión en Redis (`session:{email}`) alineado a **60 minutos**.
    *   **Comportamiento de Seguridad:** No existe renovación automática de tokens (*Zero Auto-Refresh*). Cumplidos los 60 minutos, el token expira de forma sincronizada en firma criptográfica y en caché de Redis, forzando la terminación inmediata de la sesión y requiriendo un nuevo inicio de sesión explícito (`POST /auth/login`).
*   [ ] **Revocación Activa de Sesión (`POST /auth/logout`):** Endpoint perimetral autenticado que elimina atómicamente la clave `session:{email}` en Redis ante el cierre voluntario de sesión en las aplicaciones clientes antes de cumplirse la hora.
*   [ ] **Exposición de Métricas Prometheus (`GET /metrics`):** Instrumentación perimetral para recolectar latencias (p50, p95, p99), throughput de peticiones, ratios de acierto de caché (`X-Cache: HIT/MISS`) y códigos de estado HTTP aguas abajo.
*   [ ] **Integración Continua CI/CD (`.github/workflows/ci.yml`):** Pipeline automatizado para ejecución de la suite de pruebas `go test -v ./...` y validación de build de Docker en cada push/PR.


---

## 6. Decisiones Arquitectónicas (ADRs)

Según lo estipulado en `agent.md`, toda implementación correspondiente a la Fase 2 en adelante debe ir acompañada de un **Architecture Decision Record (ADR)** ubicado en `docs/adr/`.

*   **ADRs Implementados:**
    *   [ADR-001: Migración de MySQL a PostgreSQL con asyncpg y Connection Pooling mediante PgBouncer](docs/adr/0001-migracion-postgresql-asyncpg-pgbouncer.md).
    *   [ADR-002: Estrategia de Caché de Sesiones, Proxy Caching y Rate Limiting usando Redis](docs/adr/0002-estrategia-cache-sesiones-rate-limiting-redis.md).
*   *Próximos ADRs a crear:* 
    *   `ADR-003: Trazabilidad Distribuida con OpenTelemetry (Jaeger) y Resiliencia con Circuit Breaking.`

---

## 7. Guía Canónica de Integración con Nuevos Microservicios y Proyectos

Esta sección describe el protocolo estándar para desacoplar y conectar cualquier nuevo proyecto (Frontend SPA/PWA/Mobile y Backend aguas abajo en cualquier lenguaje/framework) utilizando este Gateway como proveedor centralizado de identidad y proxy perimetral.

```
┌─────────────────────────────────┐
│   Aplicación Cliente (Frontend) │  :3000 (o app móvil)
└────────────────┬────────────────┘
                 │ 1. Auth:   POST http://localhost:8000/auth/login [X-Client-Id]
                 │ 2. Proxy:  GET/POST http://localhost:8000/<prefijo>/... [Bearer + X-Client-Id]
                 ▼
┌─────────────────────────────────┐
│       API Gateway (Go)          │  :8000
│  • Valida X-Client-Id           │
│  • Emite y valida tokens JWT    │
│  • Purgado anti-spoofing        │
│  • Inyección segura de headers  │
└────────────────┬────────────────┘
                 │ Forwarding (Streaming Zero-Copy + Cabeceras Contextuales)
                 ▼
┌─────────────────────────────────┐
│     Microservicio Downstream    │  :<puerto_servicio> (NestJS, FastAPI, Go, etc.)
│  • Valida X-Internal-Api-Key    │
│  • Lee X-User-ID y X-User-Role  │
│  • Ejecuta lógica de dominio    │
└─────────────────────────────────┘
```

---

### Paso 1: Levantar la Infraestructura Base del Gateway
El Gateway requiere PostgreSQL 16 (persistencia de usuarios y credenciales) y Redis 7 (gestión de sesiones y rate limiting):

```bash
# En el directorio raíz de api-gateway-auth:
docker-compose up -d
```
Verifica que los contenedores estén activos:
* PostgreSQL: `localhost:5432`
* Redis: `localhost:6379`

Para arrancar el Gateway en entorno de desarrollo local:
```bash
go run ./cmd/gateway
```
*El servicio escuchará por defecto en `http://localhost:8000`.*

---

### Paso 2: Registrar el Nuevo Microservicio en el Gateway (`.env`)
En el archivo `.env` del Gateway, agrega la configuración del microservicio dentro de la variable JSON `MICROSERVICES` y configura el origen permitido en `ORIGIN` para CORS:

```env
ORIGIN=http://localhost:3000

MICROSERVICES='{
  "<prefijo_servicio>": {
    "url": "http://localhost:<puerto_servicio>",
    "api_key": "<clave_secreta_compartida_interna>",
    "allowed_clients": ["<client-id-web>", "<client-id-mobile>"],
    "required_roles": ["user", "admin"],
    "public_paths": ["/api/v1/publico", "/api/v1/catalogo"],
    "allow_anonymous": false
  }
}'
```

#### Diccionario de Propiedades de Configuración:
| Clave | Tipo | Obligatorio | Descripción |
| :--- | :---: | :---: | :--- |
| `url` | `string` | **Sí** | URL base del microservicio interno de destino. |
| `api_key` | `string` | No | Secreto inyectado en `X-Internal-Api-Key` para autenticación de servicio a servicio. |
| `allowed_clients`| `[]string`| **Sí (Zero-Trust)** | Identificadores de clientes autorizados (`client_id`). Para admitir cualquier cliente use `["*"]`. |
| `required_roles` | `[]string`| No | Roles globales del Gateway exigidos para acceder al microservicio (`"user"`, `"admin"`). Si se omite, cualquier usuario autenticado tiene acceso. |
| `public_paths` | `[]string`| No | Prefijos de sub-rutas accesibles de forma anónima (sin token JWT). |
| `allow_anonymous`| `bool` | No | Si es `true`, todas las rutas del microservicio admiten tráfico anónimo. |

> [!NOTE]
> **Networking en Contenedores:** Si ejecutas el Gateway dentro de un contenedor Docker y el microservicio downstream corre en el host (máquina física), utiliza `http://host.docker.internal:<puerto>` en lugar de `http://localhost:<puerto>`.

---

### Paso 3: Consumo desde el Frontend / Aplicación Cliente
Toda aplicación consumidora debe interactuar exclusivamente con la URL base del Gateway (`http://localhost:8000`).

#### A. Autenticación y Registro Público
* **Registro de Usuario:** `POST /auth/register` (Admite campo opcional `"id"` con UUIDv4 generado por el cliente. Rol por defecto: `"user"`. Prohibido crear `"admin"`).
* **Inicio de Sesión:** `POST /auth/login`
* **Cabecera Obligatoria:** `X-Client-Id: <client-id-registrado>`

```http
POST /auth/login HTTP/1.1
Host: localhost:8000
Content-Type: application/json
X-Client-Id: mi-app-web

{
  "email": "usuario@ejemplo.com",
  "password": "Password123"
}
```
**Respuesta exitosa (`200 OK` / `201 Created`):**
```json
{
  "user": "usuario@ejemplo.com",
  "role": "user",
  "client_id": "mi-app-web",
  "token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..."
}
```

#### B. Endpoints M2M para Backends Aguas Abajo (Machine-to-Machine)
Para orquestar creación de usuarios y rollbacks atómicos (patrón Saga) entre el Gateway y las BDs de negocio:
* **Creación M2M:** `POST /auth/admin/users` (Header `X-Gateway-Secret: <GATEWAY_SECRET>`)
  ```json
  {
    "id": "550e8400-e29b-41d4-a716-446655440000",
    "email": "usuario@ejemplo.com",
    "password": "Password123!",
    "client_id": "mi-app-backend",
    "role": "user",
    "metadata": {}
  }
  ```
* **Compensación / Rollback M2M:** `DELETE /auth/admin/users/:id` (Header `X-Gateway-Secret: <GATEWAY_SECRET>`)

#### C. Consumo de Rutas Aguas Abajo (Reverse Proxy)
Todas las llamadas hacia el microservicio deben anteponer el `<prefijo_servicio>` configurado:

* **Endpoint Público (Declarado en `public_paths`):**
  ```http
  GET /<prefijo_servicio>/api/v1/catalogo HTTP/1.1
  Host: localhost:8000
  X-Client-Id: mi-app-web
  ```
* **Endpoint Protegido:**
  ```http
  GET /<prefijo_servicio>/api/v1/perfil HTTP/1.1
  Host: localhost:8000
  Authorization: Bearer <TOKEN_JWT>
  X-Client-Id: mi-app-web
  ```

---

### Paso 4: Implementación en el Microservicio Aguas Abajo (Downstream)
El microservicio interno **no almacena contraseñas, no emite tokens ni valida firmas JWT criptográficas**. Opera bajo una arquitectura perimetral *Zero-Trust*, confiando en las cabeceras contextuales inyectadas por el Gateway tras el saneamiento anti-spoofing:

#### Cabeceras Inyectadas hacia el Microservicio:
| Cabecera Recibida | Origen | Utilidad en el Microservicio |
| :--- | :--- | :--- |
| `X-Internal-Api-Key` | Configuración Gateway | Valida que la llamada proviene legítimamente del Gateway perimetral. |
| `X-User-Id` | BD / Sesión Gateway | UUID inmutable del usuario autenticado para asociar registros y propiedad. |
| `X-User-Email` | Token / Sesión | Correo verificado del usuario. |
| `X-User-Role` | Token / Sesión | Rol a nivel de sistema del Gateway (`"admin"` o `"user"`). El dominio de negocio maneja permisos finos internamente. |
| `X-Client-Id` | Token / Petición | Aplicación cliente emisora de la acción (`mi-app-web`, `mi-app-mobile`). |
| `X-Request-Id` | Gateway (Chi/Tracing) | UUID de trazabilidad correlacionada para tracing distribuido. |

#### Patrón de Guard / Middleware en el Backend Interno (Ejemplo Estándar):
```typescript
// Middleware o Guard genérico en el microservicio downstream
export function verifyGatewayPerimeter(req, res, next) {
  // 1. Validar que la petición provenga del Gateway
  const apiKey = req.headers['x-internal-api-key'];
  if (apiKey !== process.env.GATEWAY_INTERNAL_API_KEY) {
    return res.status(401).json({ error: 'Acceso no autorizado: petición fuera de perímetro' });
  }

  // 2. Extraer identidad autenticada y trazabilidad
  req.currentUser = {
    id: req.headers['x-user-id'] || null,
    email: req.headers['x-user-email'] || null,
    role: req.headers['x-user-role'] || null,
    clientId: req.headers['x-client-id'] || null,
    requestId: req.headers['x-request-id'] || null,
  };

  next();
}
```
Con este desacoplamiento, la lógica de negocio de cualquier nuevo proyecto se mantiene limpia, modular y protegida sin duplicar implementaciones de autenticación.


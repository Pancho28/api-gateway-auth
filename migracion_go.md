# 🚀 Plan de Migración de API Gateway: Python a Go (`migracion_go.md`)

**Autor:** Senior Systems Architect & Cloud Backend Specialist  
**Proyecto:** API Gateway & Centralized Auth  
**Entorno de Ejecución:** Railway (Infraestructura Cloud de Pago por Uso / Pay-per-Use)  
**Fecha:** Octubre 2026  
**Estado:** Documento de Especificación de Migración y Auditoría  

---

## 1. Resumen Ejecutivo y Motivación

Este documento establece el análisis técnico exhaustivo, la justificación económica y la hoja de ruta detallada para migrar el **API Gateway & Auth Service** desde su implementación original en Python (FastAPI) hacia un binario nativo de alto rendimiento en **Go (Golang)**.

### Objetivos Principales de la Migración
1. **Reducción de Costes en Railway (Pay-per-Use):**
   * Disminución del consumo de memoria base en reposo (*Idle RAM*) de **~100 MB a < 20 MB** (ahorro superior al 80% en costes fijos 24/7 de RAM).
   * Reducción drástica del cómputo de vCPU por petición mediante compilación nativa y goroutines multiplexadas.
2. **Eficiencia y Streaming Real Zero-Copy:**
   * Sustituir el cliente HTTP y el buffering en memoria de FastAPI/httpx por el estándar de la industria `net/http/httputil.ReverseProxy` de Go.
3. **Eliminación de Deuda Técnica y Vulnerabilidades Arquitectónicas:**
   * Subsanar problemas críticos detectados en la versión de Python: fugas de sockets, carga completa de respuestas `GET` en memoria RAM, consumo innecesario de conexiones a la base de datos por petición proxy y falta de propagación de cabeceras de identidad hacia microservicios internos.

---

## 2. Auditoría Técnica: Puntos Vulnerables y Defectos de la Versión Python

Durante la auditoría del código real en `gateway-python`, se detectaron siete (7) puntos vulnerables y deficiencias arquitectónicas que comprometen el rendimiento, la estabilidad y la seguridad del sistema:

---

### ⚠️ Vulnerabilidad 1: Ruptura de Streaming y Picos de Memoria RAM en peticiones `GET`
* **Ubicación:** [`gateway-python/services/proxy_service.py` (Líneas 140–169)](file:///c:/Users/franc/OneDrive/Desktop/Pancho/Proyectos/api-gateway-auth/gateway-python/services/proxy_service.py#L140-L169)
* **Mecanismo:**
  Para almacenar en caché las respuestas `GET` en Redis, el Gateway ejecuta:
  ```python
  if hasattr(resp, "aread") and callable(resp.aread):
      body_bytes = await read_res
  elif hasattr(resp, "aiter_raw"):
      body_bytes = b"".join([chunk async for chunk in resp.aiter_raw()])
  
  # Decodificación completa a string y serialización en memoria
  payload = {"body": body_bytes.decode("utf-8", errors="replace"), ...}
  ```
* **Impacto:**
  1. Se anula el streaming HTTP. La respuesta del microservicio se descarga en su totalidad antes de comenzar a enviarse al cliente final.
  2. La memoria RAM se triplica en el heap de Python: un buffer de bytes + un string decodificado + el objeto serializado JSON para Redis.
  3. Si un microservicio devuelve un archivo grande (PDF, imagen, CSV o JSON voluminoso de >10 MB), múltiples peticiones concurrentes dispararán un **OOM (Out-of-Memory Killer)** en el contenedor de Railway.
* **Solución en Go:**
  El paquete `net/http/httputil.ReverseProxy` transmite en streaming directo mediante `io.Copy` con buffers fijos de 32 KB. Para el almacenamiento en caché, se empleará un `io.TeeReader` que solo guarde respuestas si el tamaño total es menor a un umbral predefinido (ej. 1 MB). Si supera dicho límite, se transmite en streaming directo al cliente sin bufferizar en memoria ni guardar en Redis.

---

### ⚠️ Vulnerabilidad 2: Potencial Fuga de Conexiones en Streaming de Peticiones Mutables
* **Ubicación:** [`gateway-python/services/proxy_service.py` (Líneas 173–178)](file:///c:/Users/franc/OneDrive/Desktop/Pancho/Proyectos/api-gateway-auth/gateway-python/services/proxy_service.py#L173-L178)
* **Mecanismo:**
  ```python
  return StreamingResponse(
      resp.aiter_raw(),
      status_code=resp.status_code,
      headers=dict(resp.headers),
      background=None # <--- No se pasa tarea de cierre
  )
  ```
* **Impacto:**
  En `httpx`, cuando una petición se abre con `stream=True`, la conexión permanece tomada del pool hasta que la respuesta se consume por completo o se cierra explícitamente (`resp.aclose()`). Si un cliente se desconecta intempestivamente a mitad de la transferencia (cliente cancela petición o pérdida de señal), Starlette interrumpe el generador pero `resp.aclose()` nunca se llama. Bajo tráfico continuo o clientes inestables, esto agota las conexiones activas del pool de `httpx`, devolviendo errores `502 Bad Gateway` o timeouts.
* **Solución en Go:**
  `httputil.ReverseProxy` maneja automáticamente los contextos (`context.Context`), la cancelación por desconexión del cliente (`r.Context().Done()`) y el reciclaje higiénico de los descriptores de socket en el `http.Transport`.

---

### ⚠️ Vulnerabilidad 3: Adquisición Innecesaria de Conexiones de DB en el Flujo Proxy
* **Ubicación:** [`gateway-python/api/dependencies.py` (Líneas 17–21)](file:///c:/Users/franc/OneDrive/Desktop/Pancho/Proyectos/api-gateway-auth/gateway-python/api/dependencies.py#L17-L21) y [`gateway-python/api/routers/gateway.py` (Línea 27)](file:///c:/Users/franc/OneDrive/Desktop/Pancho/Proyectos/api-gateway-auth/gateway-python/api/routers/gateway.py#L27)
* **Mecanismo:**
  La ruta `reverse_proxy` inyecta `current_user = Depends(get_current_user)`. A su vez, `get_current_user` inyecta:
  ```python
  async def get_current_user(
      request: Request,
      token: str = Depends(oauth2_scheme),
      db: AsyncSession = Depends(get_db) # <--- Ejecutado ANTES de verificar Redis
  )
  ```
* **Impacto:**
  El inyector de dependencias de FastAPI ejecuta el generador `get_db()` antes de entrar al cuerpo de la función. Por tanto, **en el 100% de las peticiones que pasan por el Proxy**, se reserva una sesión de base de datos y se toma una conexión del pool (`asyncpg` / `SQLAlchemy` con `pool_pre_ping=True`), **incluso cuando la sesión del usuario está en Redis y la base de datos relacional nunca se consulta**.
  Bajo concurrencia de 500 req/s, el pool local de SQLAlchemy colapsa innecesariamente (`TimeoutError: QueuePool limit reached`), a pesar de haber diseñado la capa de caché en Redis precisamente para proteger la base de datos.
* **Solución en Go:**
  El middleware de autenticación (`AuthMiddleware`) consultará exclusivamente Redis para validar la sesión y el rol. Únicamente si la clave no existe en Redis (*Cache Miss*), se solicitará una conexión puntual desde el pool `pgxpool` para consultar PostgreSQL y repoblar Redis.

---

### ⚠️ Vulnerabilidad 4: Falta de Propagación de Identidad hacia los Microservicios
* **Ubicación:** [`gateway-python/services/proxy_service.py` (Líneas 112–121)](file:///c:/Users/franc/OneDrive/Desktop/Pancho/Proyectos/api-gateway-auth/gateway-python/services/proxy_service.py#L112-L121)
* **Mecanismo:**
  `proxy_service.forward_request(request, path)` recibe la petición entrante, purga las cabeceras presentes en `BLOCKED_HEADERS` (`x-user-id`, `x-user-email`, `x-user-role`) para evitar *Header Spoofing*, pero **nunca recibe ni inyecta los datos del usuario autenticado** (`current_user`).
* **Impacto:**
  El Gateway valida exitosamente quién es el usuario, pero reenvía la petición al microservicio interno desprovista de cualquier dato de identidad. Los microservicios aguas abajo quedan ciegos respecto a quién ejecutó la acción, forzando a que cada microservicio deba recibir el token `Bearer`, parsearlo de nuevo y validar su firma, duplicando el trabajo de seguridad y rompiendo el principio del Gateway como autenticador centralizado.
* **Solución en Go:**
  El middleware colocará el struct `UserClaims` en el `context.Context` de la petición. Cuando `ReverseProxy` reescriba las cabeceras, purgará las cabeceras inseguras del cliente e inyectará de forma fidedigna y segura:
  * `X-User-ID: <uuid>`
  * `X-User-Email: <email>`
  * `X-User-Role: <ADMIN|USER>`
  * `X-Internal-Api-Key: <secret>`

---

### ⚠️ Vulnerabilidad 5: Desfase de Evaluación en Slowapi Rate Limiting vs Contexto de Usuario
* **Ubicación:** [`gateway-python/core/rate_limit.py` (Líneas 11–20)](file:///c:/Users/franc/OneDrive/Desktop/Pancho/Proyectos/api-gateway-auth/gateway-python/core/rate_limit.py#L11-L20)
* **Mecanismo:**
  ```python
  def custom_key_func(request: Request) -> str:
      if hasattr(request.state, "user") and request.state.user is not None:
          return str(request.state.user.email)
      return get_remote_address(request)
  ```
* **Impacto:**
  En FastAPI, el decorador `@limiter.limit` de `slowapi` se evalúa al inicio de la ruta, antes de que las dependencias de FastAPI (como `get_current_user`) asignen `request.state.user`. Como consecuencia, `request.state.user` es siempre `None` en el momento de calcular la cuota, provocando que el rate limiter **siempre haga fallback a la dirección IP remota**. En entornos detrás de balanceadores o Proxies Ingress de Railway, múltiples usuarios reales comparten la misma IP de entrada, provocando bloqueos involuntarios a usuarios legítimos.
* **Solución en Go:**
  El middleware de rate limiting (`redis_rate`) se ejecuta **después** del middleware de autenticación en la cadena de middlewares de `chi`. De esta manera, el usuario ya ha sido identificado y autenticado en el contexto, permitiendo un rate limiting estricto por `User ID` o por `IP` si la ruta es pública.

---

### ⚠️ Vulnerabilidad 6: Bloqueo Síncrono de Sockets durante la Importación del Módulo
* **Ubicación:** [`gateway-python/core/rate_limit.py` (Líneas 26–32)](file:///c:/Users/franc/OneDrive/Desktop/Pancho/Proyectos/api-gateway-auth/gateway-python/core/rate_limit.py#L26-L32)
* **Mecanismo:**
  ```python
  def get_rate_limiter_storage_uri() -> str:
      try:
          with socket.create_connection((settings.REDIS_HOST, settings.REDIS_PORT), timeout=0.1):
              return settings.REDIS_URL
      except (OSError, ConnectionRefusedError):
          return "memory://"

  limiter = Limiter(storage_uri=get_rate_limiter_storage_uri(), ...)
  ```
* **Impacto:**
  Esta llamada a socket síncrona y bloqueante se ejecuta a nivel de módulo durante el `import core.rate_limit`. Si la red de Redis tiene una latencia de handshake superior a 100 ms o si el DNS de Railway tarda en resolver el hostname de Redis al arrancar, el import conmuta silenciosamente a `memory://`, perdiendo el rate limiting distribuido sin notificar adecuadamente.
* **Solución en Go:**
  La inicialización de clientes se realiza de forma explícita y asíncrona dentro del ciclo de inicio (`main()`), con reintentos controlados y validación mediante `client.Ping(ctx)`.

---

### ⚠️ Vulnerabilidad 7: Uso de API Deprecada `datetime.utcnow()`
* **Ubicación:** [`gateway-python/core/security.py` (Líneas 23 y 25)](file:///c:/Users/franc/OneDrive/Desktop/Pancho/Proyectos/api-gateway-auth/gateway-python/core/security.py#L23-L25)
* **Impacto:**
  En Python 3.12+, `datetime.utcnow()` está formalmente deprecada por retornar objetos naive (sin zona horaria explícita), lo que genera `DeprecationWarning` y potenciales inconsistencias en cálculo de expiraciones de tokens si el sistema operativo local o contenedor maneja offsets de zona horaria diferentes a UTC.
* **Solución en Go:**
  Go maneja tiempos estrictos con `time.Now().UTC()`, eliminando ambigüedades de zona horaria.

---

## 3. Comparativa de Recursos y Costes: Python vs. Go vs. Rust

| Métrica / Dimensión | Python 3.13 (FastAPI) | Go 1.24 (`chi` + `httputil`) | Rust (`axum` + `hyper`) |
| :--- | :--- | :--- | :--- |
| **Huella RAM Base (Idle)** | 85 MB – 140 MB por worker | **15 MB – 22 MB** | 8 MB – 15 MB |
| **RAM bajo 1,000 req/s** | 250 MB – 500 MB (GC dependiente) | **30 MB – 50 MB** | 18 MB – 35 MB |
| **Latencia adicional de Gateway**| 4.0 ms – 12.0 ms | **0.3 ms – 0.8 ms** | 0.1 ms – 0.4 ms |
| **Capacidad I/O (1 vCPU)** | ~2,000 – 3,500 req/s | **~35,000 – 50,000 req/s** | ~65,000+ req/s |
| **Tamaño de Imagen Docker** | ~180 MB – 250 MB | **< 20 MB** (Alpine / Scratch) | < 25 MB |
| **Coste estimado en Railway (Mes)**| ~$2.50 – $5.00 USD | **~$0.20 – $0.40 USD** | ~$0.10 – $0.25 USD |
| **Complejidad de Mantenimiento** | Baja | **Baja (Idiomática)** | Alta (Lifetimes, async traits) |
| **Velocidad de Compilación en CI** | Inmediata (interpretado) | **~5 – 10 segundos** | ~2 – 4 minutos |

### Veredicto del Trade-off Go vs. Rust:
* Rust ofrece un margen microscópico adicional de rendimiento sobre Go (~10 MB menos de RAM y 0.2 ms menos de latencia).
* Sin embargo, en Railway, **10 MB de RAM representan menos de $0.10 USD al mes de diferencia**.
* A cambio, Rust exige una curva de desarrollo y mantenimiento mucho más compleja para proxies HTTP (`tower`, `hyper 1.0`, pinning de streams asíncronos), junto a builds largos en Railway.
* **Go es el balance perfecto:** Rendimiento nativo de primer nivel, biblioteca estándar diseñada para redes y la menor fricción de mantenimiento.

---

## 4. Arquitectura Objetivo en Go (`gateway-go`)

La estructura de carpetas en `gateway-go` seguirá las mejores prácticas estándar de la comunidad Go (*Standard Go Project Layout*):

```
gateway-go/
├── cmd/
│   └── gateway/
│       └── main.go                 # Entrada de la aplicación y ciclo de vida (Graceful Shutdown)
├── internal/
│   ├── config/
│   │   └── config.go               # Carga de variables de entorno con fallback
│   ├── database/
│   │   ├── postgres.go             # Pool de conexiones pgxpool compatible con PgBouncer
│   │   └── seed.go                 # Seeding del usuario admin al iniciar
│   ├── redis/
│   │   └── redis.go                # Cliente go-redis singleton con degradación elegante
│   ├── models/
│   │   └── user.go                 # Estructura del usuario y DTOs
│   ├── auth/
│   │   ├── password.go             # Hashing PBKDF2_SHA256 (compatible 100% con Python)
│   │   ├── jwt.go                  # Emisión y validación de tokens JWT
│   │   └── service.go              # Lógica de login y gestión de sesiones
│   ├── user/
│   │   └── service.go              # Lógica de creación de usuarios administrados
│   ├── proxy/
│   │   ├── service.go              # httputil.ReverseProxy con sanitización y cabeceras
│   │   └── cache.go                # Interceptor de caché Redis para GET con límite de tamaño
│   └── api/
│       ├── router.go               # Definición de rutas y montaje de Chi
│       ├── middleware/
│       │   ├── auth.go             # Validación JWT + Redis sin tocar PostgreSQL
│       │   ├── cors.go             # Manejo estricto de CORS
│       │   ├── rate_limit.go       # Limitador distribuido con redis_rate por User ID / IP
│       │   └── sanitize.go         # Protección contra Path Traversal
│       └── handlers/
│           ├── auth_handler.go     # Handler HTTP para /auth/login
│           ├── user_handler.go     # Handler HTTP para /users
│           └── gateway_handler.go  # Catch-all handler para reenvío de microservicios
├── test/                           # Pruebas de integración
├── Dockerfile                      # Multi-stage build (scratch/alpine)
├── go.mod
└── go.sum
```

---

## 5. Matriz de Dependencias Seleccionadas

| Funcionalidad | Librería en Go | Justificación Técnica |
| :--- | :--- | :--- |
| **Router HTTP** | `github.com/go-chi/chi/v5` | Extremadamente rápido, 100% compatible con `http.Handler`, sin dependencias externas, soporte idiomático de sub-routers y middlewares. |
| **Reverse Proxy** | `net/http/httputil` | Biblioteca estándar de Go. Provee streaming seguro, reciclaje de conexiones TCP, soporte HTTP/2 y control de headers hop-by-hop. |
| **Driver PostgreSQL** | `github.com/jackc/pgx/v5/pgxpool` | Driver de más alto rendimiento en Go. Compatible con transaction pooling de PgBouncer configurando `QueryExecModeExec`. |
| **Cliente Redis** | `github.com/redis/go-redis/v9` | Cliente oficial de Redis con connection pooling automático, pipeline y reintentos. |
| **Rate Limiter** | `github.com/go-redis/redis_rate/v10` | Implementación del algoritmo *Generic Cell Rate Algorithm* (GCRA) sobre Redis, ultra-eficiente y distribuido. |
| **Tokens JWT** | `github.com/golang-jwt/jwt/v5` | Estándar de la industria en Go para emisión y validación de claims HS256/RS256. |
| **Password Hashing** | `golang.org/x/crypto/pbkdf2` | Permite verificar de forma transparente los hashes creados por Passlib en Python (`pbkdf2_sha256`) sin romper credenciales existentes. |
| **Validación de Datos** | `github.com/go-playground/validator/v10` | Equivalente en Go para las validaciones declarativas de Pydantic (regex, longitudes mínimas/máximas, emails). |

---

## 6. Plan de Acción y Hoja de Ruta de Migración

```
Fase 1: Estructura & Core          Fase 2: Auth & Redis            Fase 3: Proxying & Streaming          Fase 4: Tests & Despliegue
  • go mod init                     • Hashing compatible            • httputil.ReverseProxy               • Portar 59 tests
  • config & pgxpool                • JWT Claims & Handlers         • Inyección de Headers                • Dockerfile Alpine
  • Seeding Admin                   • Sesiones en Redis             • Límite seguro de Caché GET          • Validación en Railway
```

### Paso 1: Configuración Base y Acceso a Datos
1. Inicializar módulo `go mod init api-gateway-go` dentro de `gateway-go/`.
2. Implementar carga de configuración desde variables de entorno con paridad a `.env`.
3. Conectar a PostgreSQL mediante `pgxpool` y ejecutar la creación o verificación de la tabla `users`.
4. Implementar el *seeding* del usuario administrador al arrancar.

### Paso 2: Autenticación, Hashing y Sesiones en Redis
1. Implementar la función de verificación PBKDF2 compatible con los hashes generados por `passlib`.
2. Implementar generación de JWT y endpoint `/auth/login`.
3. Almacenar sesión en Redis tras login exitoso (`session:{email}`).
4. Crear middleware `AuthMiddleware` que verifique token y valide sesión en Redis en **<0.5 ms** sin consultar PostgreSQL.

### Paso 3: Reverse Proxying y Middlewares
1. Implementar `httputil.ReverseProxy` con función `Rewrite`.
2. Aplicar validación de ruta y prevención de *Path Traversal*.
3. Purgar cabeceras hop-by-hop e inyectar cabeceras de contexto (`X-User-ID`, `X-User-Email`, `X-User-Role`, `X-Internal-Api-Key`).
4. Implementar caché de peticiones `GET` con tamaño acotado (máximo 1 MB) para evitar saturación de memoria.
5. Configurar limitación de tasa distribuida con `redis_rate`.

### Paso 4: Suite de Pruebas y Contenedorización
1. Portar los 59 casos de prueba existentes utilizando `net/http/httptest` y `github.com/stretchr/testify`.
2. Crear `Dockerfile` multi-stage para producción.
3. Desplegar en Railway verificando reducción de huella de memoria y tiempos de respuesta.

---

## 7. Estado de la Organización de Carpetas

A la fecha de emisión de este documento, el repositorio ha quedado estructurado de la siguiente forma:

* **`/` (Raíz):**
  * `migracion_go.md` (Este documento).
  * `README.md`, `SPEC_API_GATEWAY_AUTH.md`, `agent.md`.
  * `docs/` (ADR-001 y ADR-002).
  * `docker-compose.yml` (Infraestructura compartida: Postgres 16, PgBouncer, Redis 7).
  * `.env.sample`.
* **`gateway-python/`:**
  * Proyecto completo original en Python (FastAPI, SQLAlchemy, servicios, routers y los 59 tests verificados al 100%).
* **`gateway-go/`:**
  * Directorio preparado para la construcción del nuevo servicio en Go.

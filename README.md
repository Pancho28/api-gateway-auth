# 🚀 API Gateway & Centralized Auth Service (Go 1.24)

Servicio de **API Gateway & Identidad Centralizada (IdP)** de alto rendimiento desarrollado en **Go**, diseñado para arquitecturas de microservicios y despliegues *pay-per-use* en **Railway**.

Ofrece **latencia perimetral sub-milisegundo (<1 ms)**, **streaming real zero-copy**, aislamiento **Zero-Trust** por cliente (`allowed_clients`), filtrado configurable de cabeceras de contexto (`passthrough_headers`) y una huella de memoria de **~18 MB de RAM**.

---

## 📦 Stack Tecnológico
* **Router & Middlewares:** `[github.com/go-chi/chi/v5](https://github.com/go-chi/chi/v5)` + `[github.com/go-chi/cors](https://github.com/go-chi/cors)`
* **Reverse Proxy:** `net/http/httputil.ReverseProxy` (Streaming nativo HTTP/1.1 y HTTP/2)
* **Base de Datos:** PostgreSQL 16 (`pgxpool` en Simple Protocol optimizado para PgBouncer)
* **Caché, Sesiones & Rate Limiting:** Redis 7 (`go-redis/v9` + `redis_rate/v10`)
* **Seguridad & Tokens:** `golang-jwt/jwt/v5` y hashing PBKDF2 (`golang.org/x/crypto/pbkdf2`)
* **Testing:** Suite estándar de Go con `[github.com/stretchr/testify](https://github.com/stretchr/testify)`

---

## 📁 Estructura del Proyecto

```text
api-gateway-auth/
├── cmd/gateway/main.go            # Punto de entrada de la aplicación
├── internal/
│   ├── api/                       # Router Chi, middlewares y handlers HTTP
│   ├── auth/                      # Sesiones en Redis y autenticación
│   ├── config/                    # Configuración de entorno y reglas Zero-Trust
│   ├── database/                  # Pool PostgreSQL (pgxpool) y repositorios
│   ├── models/                    # Structs de usuario, claims JWT y DTOs
│   ├── proxy/                     # Reverse proxy, anti-spoofing, passthrough_headers y caché
│   ├── redis/                     # Cliente y wrappers de Redis
│   ├── security/                  # Hashing PBKDF2 y ciclo de vida JWT
│   └── user/                      # Creación de usuarios y endpoints M2M
├── test/api_test.go               # Suite de pruebas de integración perimetrales
├── debug/                         # Directorio exclusivo para scripts temporales (ignorado en Git)
├── docs/adr/                      # Architecture Decision Records
├── SPEC_API_GATEWAY_AUTH.md       # Especificación Técnica Canónica (SDD)
├── PLAN.md                        # Plan de ejecución activo (temporal, ignorado en Git)
├── TODO.md                        # Backlog centralizado y hoja de ruta pendiente
├── agent.md                       # Directivas operativas para Asistentes de IA
└── docker-compose.yml             # PostgreSQL 16, PgBouncer, Redis 7 y Jaeger
```

---

## 🛠️ Comandos Rápidos

```bash
# 1. Levantar infraestructura (Postgres 16, PgBouncer, Redis 7, Jaeger)
docker-compose up --build -d

# 2. Ejecutar Gateway en desarrollo local (:8000)
go run ./cmd/gateway

# 3. Ejecutar suite completa de pruebas (TDD)
go test -v ./...

# 4. Compilar binario optimizado de producción
go build -ldflags="-s -w" -o gateway ./cmd/gateway
```

---

## 🔒 Configuración de Microservicios (`MICROSERVICES` en `.env`)

El Gateway aplica una política **Zero-Trust (Deny by Default)** tanto en el acceso de clientes como en el paso de cabeceras HTTP:

```env
MICROSERVICES='{
    "reservas": {
        "url": "http://localhost:4000",
        "api_key": "secret-key-reservas-123",
        "allowed_clients": ["reservas-web", "hostess-pwa"],
        "public_paths": ["/api/v1/marketplace", "/api/v1/auth/register", "/api/v1/reservations", "/api/v1/reviews"],
        "passthrough_headers": ["X-Restaurant-Id", "X-Timezone"]
    },
    "ventas": {
        "url": "http://localhost:8001",
        "api_key": "secret-key-ventas-123",
        "allowed_clients": ["comensales-app", "staff-pos", "admin-portal"]
    },
    "stock": {
        "url": "http://localhost:8002",
        "api_key": "secret-key-stock-456",
        "allowed_clients": ["staff-pos", "admin-portal"],
        "required_roles": ["user", "admin"]
    }
}'
```

| Propiedad | Obligatorio | Descripción |
| :--- | :---: | :--- |
| `url` | **Sí** | URL interna del microservicio destino. |
| `api_key` | No | Secreto inyectado en `X-Internal-Api-Key`. |
| `allowed_clients` | **Sí** | Lista de `client_id` autorizados (o `["*"]`). Si se omite, bloquea con `403 [E-03]`. |
| `required_roles` | No | Roles del Gateway permitidos (`"user"`, `"admin"`). |
| `public_paths` | No | Prefijos de sub-rutas públicas que no exigen JWT. |
| `allow_anonymous` | No | Si es `true`, permite tráfico anónimo en todo el servicio. |
| `passthrough_headers`| No | Lista blanca de cabeceras `X-*` del cliente que el Gateway permite reenviar al servicio. Toda otra cabecera `X-*` no estándar es purgada por seguridad. |

---

## 🛡️ Cabeceras y Contratos Perimetrales

1. **Cabeceras Inyectadas por el Gateway (Inmutables):**
   - `X-Internal-Api-Key`: Secreto compartido del servicio destino.
   - `X-User-Id`: UUIDv4 inmutable del usuario autenticado.
   - `X-User-Email`: Correo electrónico verificado.
   - `X-User-Role`: Rol global en el Gateway (`"user"` o `"admin"`).
   - `X-Client-Id`: Identificador verificado de la app cliente.
   - `X-Request-Id`: UUID de trazabilidad.
2. **Endpoints Principales de Identidad:**
   - `POST /auth/register`: Registro público de usuarios (`role: "user"`, admite `"id"` UUIDv4 opcional generado por el cliente).
   - `POST /auth/login`: Inicio de sesión con `X-Client-Id` obligatorio (sesión de 60 min en Redis).
   - `POST /auth/logout`: Cierre de sesión e invalidación inmediata de la clave en Redis.
   - `POST /auth/admin/users` y `DELETE /auth/admin/users/:id`: Creación y compensación M2M protegidas por `X-Gateway-Secret`.

> Para consultar el detalle arquitectónico, las reglas de anti-spoofing y el **Catálogo Oficial de Errores (`[E-01]` a `[E-99]`)**, revisa **`SPEC_API_GATEWAY_AUTH.md`**.  
> Para consultar las tareas pendientes, mejoras en desarrollo y la hoja de ruta activa, revisa **[TODO.md](TODO.md)**.
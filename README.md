# 🚀 API Gateway & Centralized Auth Service

Servicio de **API Gateway & Identidad Centralizada** de alto rendimiento desarrollado en **Go (Golang 1.24)**, diseñado para ecosistemas de microservicios y despliegues en la nube de pago por uso (*pay-per-use*) en **Railway**.

Proporciona **latencia perimetral sub-milisegundo (<1 ms)**, **streaming real zero-copy**, arquitectura **Zero-Trust de aislamiento de clientes** (*Client-to-Service Partitioning*) y una huella de memoria ultra-reducida de **~18 MB de RAM** en reposo.

---

## 📦 Stack Tecnológico
* **Router & Middlewares:** `github.com/go-chi/chi/v5` + `github.com/go-chi/cors`
* **Reverse Proxy:** `net/http/httputil.ReverseProxy` (Streaming nativo de alto rendimiento HTTP/1.1 y HTTP/2)
* **Base de Datos:** PostgreSQL 16 con `github.com/jackc/pgx/v5/pgxpool` en Simple Protocol (optimizado para PgBouncer en Transaction Mode)
* **Caché & Estado:** Redis 7 con `github.com/redis/go-redis/v9`
* **Rate Limiting Distribuido:** `github.com/go-redis/redis_rate/v10`
* **Seguridad & Tokens:** `github.com/golang-jwt/jwt/v5` y hashing PBKDF2 compatible con Passlib (`golang.org/x/crypto/pbkdf2`)
* **Testing:** Suite estándar de Go con `github.com/stretchr/testify`

---

## 📁 Estructura del Proyecto

```
api-gateway-auth/
├── cmd/
│   └── gateway/
│       └── main.go           # Punto de entrada de la aplicación Go
├── internal/                 # Paquetes internos de la aplicación
│   ├── api/                  # Configuración de Chi, middlewares y handlers HTTP
│   ├── auth/                 # Servicio de autenticación y gestión de sesiones
│   ├── config/               # Carga y validación de variables de entorno y microservicios
│   ├── database/             # Pool de PostgreSQL (pgxpool) y repositorios
│   ├── models/               # Modelos de usuario, claims JWT y DTOs de petición
│   ├── proxy/                # Reverse proxy, saneamiento de rutas, anti-spoofing y caché
│   ├── redis/                # Cliente y wrappers de Redis
│   ├── security/             # Hashing PBKDF2 y ciclo de vida de tokens JWT
│   └── user/                 # Servicio de creación y administración de usuarios
├── test/
│   └── api_test.go           # Suite de pruebas de integración HTTP perimetrales
├── docs/                     # 📚 Architecture Decision Records (ADRs)
│   └── adr/
├── Dockerfile                # Multi-stage build para despliegue en Railway (<20 MB)
├── docker-compose.yml        # 🐳 Infraestructura (PostgreSQL 16, PgBouncer, Redis 7)
├── .env.sample               # Plantilla de variables de entorno
├── .gitignore
├── go.mod
├── go.sum
├── README.md
├── SPEC_API_GATEWAY_AUTH.md
└── agent.md                  # Reglas de desarrollo y directivas para agentes de IA
```

---

## 🛠️ Comandos Principales

### 1. Iniciar Todo el Stack con Docker Compose
Construye y levanta el API Gateway junto con PostgreSQL 16, PgBouncer y Redis 7:
```bash
docker-compose up --build -d
```

| Contenedor | Puerto Local | Función |
| :--- | :--- | :--- |
| **API Gateway (Go)** | `8000` | Gateway perimetral, proxy inverso y autenticación |
| **PostgreSQL 16** | `5432` | Base de datos persistente (usuarios y roles) |
| **PgBouncer** | `6432` | Connection Pooler de alto rendimiento |
| **Redis 7** | `6379` | Caché de sesiones JWT, proxy caching y cuotas de rate limiting |
| **Jaeger All-in-One** | `4318` / `16686` | Telemetría OTLP y Dashboard web de tracing distribuido |

### 2. Ejecución Local en Desarrollo
```bash
go run ./cmd/gateway
```
*El Gateway arrancará escuchando en el puerto `:8000` (o el configurado en `PORT`).*

### 3. Ejecutar Suite Completa de Pruebas
```bash
go test -v ./...
```

### 4. Compilación del Binario de Producción
```bash
# Compilar binario nativo optimizado sin símbolos de depuración (-s -w):
go build -ldflags="-s -w" -o gateway ./cmd/gateway

# O construir la imagen Docker para producción en Railway:
docker build -t api-gateway .
```

---

## 🧪 Metodología de Desarrollo: Test-Driven Development (TDD)
El ciclo de ingeniería de este Gateway sigue de forma estricta el estándar **TDD (Red -> Green -> Refactor)**:
1. **Fase Red (Escribir Pruebas Primero):** Antes de modificar o añadir código fuente, se deben crear o actualizar las pruebas en `*_test.go` o en `test/api_test.go` que comprueben la nueva funcionalidad o caso de fallo.
2. **Fase Green (Hacer Pasar la Prueba):** Implementar la solución mínima, atómica e idiomática en Go para que las pruebas pasen.
3. **Fase Refactor (Optimizar y Limpiar):** Mejorar legibilidad, allocations de memoria y rendimiento asegurando que el 100% de la suite de pruebas permanezca en verde (`go test -v ./...`).

---

## 🛡️ Mejoras Arquitectónicas y de Rendimiento
1. **Streaming Real Zero-Copy:** `httputil.ReverseProxy` transfiere datos en chunks mediante streaming continuo sin almacenar el cuerpo completo en la memoria RAM del Gateway.
2. **Caché GET Acotada (Protección OOM):** Se limita el cacheo automático en Redis a respuestas de máximo 1 MB particionadas por cliente (`proxy_cache:<service>:<client_id>:<hash>`).
3. **Propagación Segura & Anti-Spoofing:** El Gateway purga cabeceras sensibles no confiables enviadas por el cliente final e inyecta de forma fidedigna `X-Client-ID`, `X-Scopes`, `X-User-ID`, `X-User-Email`, `X-User-Role` y `X-Internal-Api-Key` hacia los microservicios internos.
4. **Desacoplamiento Total de Base de Datos en Proxy:** El middleware de autenticación valida la sesión en Redis (<0.5 ms). No se solicita ninguna conexión a PostgreSQL salvo en caso de *Cache Miss*.
5. **Huella de Memoria Minimizada:** Consumo base en reposo de **~18 MB de RAM** (frente a los ~140 MB de arquitecturas tradicionales en Python).
6. **Aislamiento y Particionamiento de Clientes (Zero-Trust):** Control estricto de acceso cliente-a-servicio con enlace criptográfico (*Token Binding*).

---

## 🔒 Manual de Uso: Particionamiento y Aislamiento de Clientes (Client Partitioning)

El Gateway incorpora una arquitectura de **Zero-Trust Perimetral** para garantizar que tokens emitidos para una aplicación cliente específica (ej. comensales) no puedan utilizarse contra microservicios de uso restringido (ej. staff, finanzas, cocina).

### 1. Política Zero-Trust (*Deny by Default*)
* **Regla Estricta:** Todo microservicio configurado en `MICROSERVICES` debe declarar explícitamente qué clientes tienen autorización en `allowed_clients`.
* Si un microservicio se registra con `allowed_clients` vacío u omitido, el API Gateway **bloquea el acceso por defecto con `403 Forbidden` (`[E-03]`)**.
* Para permitir cualquier cliente autenticado de forma deliberada, use el comodín `["*"]`.

### 2. Configuración de `MICROSERVICES` (Variable de Entorno)
La variable de entorno `MICROSERVICES` contiene un JSON donde cada clave representa el prefijo de ruta perimetral:

```json
{
  "ventas": {
    "url": "http://localhost:8001",
    "api_key": "secret-key-ventas-123",
    "allowed_clients": ["comensales-app", "staff-pos", "admin-portal"]
  },
  "stock": {
    "url": "http://localhost:8002",
    "api_key": "secret-key-stock-456",
    "allowed_clients": ["staff-pos", "admin-portal"],
    "required_roles": ["USER", "ADMIN"]
  },
  "cocina": {
    "url": "http://localhost:8003",
    "api_key": "secret-key-cocina-789",
    "allowed_clients": ["staff-pos", "kds-screen"]
  }
}
```

| Campo | Tipo | Obligatorio | Descripción |
| :--- | :--- | :---: | :--- |
| `url` | `string` | Sí | URL base del microservicio interno. |
| `api_key` | `string` | No | Clave interna compartida inyectada en `X-Internal-Api-Key`. |
| `allowed_clients` | `[]string` | **Sí (Zero-Trust)** | Lista de `client_id` autorizados (o `["*"]`). |
| `required_roles` | `[]string` | No | Lista opcional de roles permitidos del Gateway (`"user"`, `"admin"`). |
| `public_paths` | `[]string` | No | Lista de prefijos de rutas con acceso público/anónimo (ej. `["/marketplace", "/cities"]` o `["*"]`). |
| `allow_anonymous`| `bool` | No | Si es `true`, todo el microservicio admite peticiones anónimas sin token. |

---

### 3. Registro e Inicio de Sesión de Clientes

#### A. Registro Público de Usuarios (`POST /auth/register`)
Permite a los usuarios finales darse de alta en la plataforma. Por diseño como IdP perimetral agnóstico:
* Asigna automáticamente el rol `"user"`.
* **Prohibición Estricta:** Queda estrictamente PROHIBIDO auto-asignarse el rol `"admin"` por cualquier API. El rol `"admin"` solo se aprovisiona por variables de entorno o seeds del sistema.
* **UUID Generado por el Cliente (Client-Generated UUID):** Admite un campo opcional `"id"` (UUIDv4). Si el cliente lo envía, el Gateway persiste ese UUID exacto; si se omite, el Gateway genera uno automáticamente.

```http
POST /auth/register HTTP/1.1
Host: localhost:8000
Content-Type: application/json
X-Client-Id: reservas-web

{
  "id": "550e8400-e29b-41d4-a716-446655440000",
  "email": "usuario@ejemplo.com",
  "password": "Password123"
}
```
*Respuesta `201 Created` idéntica a `/auth/login` con token JWT emitido inmediatamente.*

#### B. Endpoints de Creación y Compensación para Backends (Machine-to-Machine M2M)
Diseñados para que los backends de las aplicaciones cliente registren usuarios de forma coordinada con sus propias bases de datos y gestionen transacciones distribuidas (Saga/Compensación):

##### 1. Creación M2M (`POST /auth/admin/users`)
Protegido mediante cabecera de secreto compartido `X-Gateway-Secret`:
```http
POST /auth/admin/users HTTP/1.1
Host: localhost:8000
Content-Type: application/json
X-Gateway-Secret: <GATEWAY_SECRET>

{
  "id": "550e8400-e29b-41d4-a716-446655440000",
  "email": "usuario@ejemplo.com",
  "password": "Password123!",
  "client_id": "pos-backend",
  "role": "user",
  "metadata": {}
}
```
**Respuesta `201 Created`:**
```json
{
  "id": "550e8400-e29b-41d4-a716-446655440000",
  "email": "usuario@ejemplo.com",
  "role": "user",
  "client_id": "pos-backend"
}
```

##### 2. Rollback / Compensación M2M (`DELETE /auth/admin/users/:id`)
Protegido mediante cabecera `X-Gateway-Secret`. Si el backend cliente sufre un fallo en su propia base de datos tras crear la credencial, llama a este endpoint para eliminar la credencial huérfana de forma atómica:
```http
DELETE /auth/admin/users/550e8400-e29b-41d4-a716-446655440000 HTTP/1.1
Host: localhost:8000
X-Gateway-Secret: <GATEWAY_SECRET>
```
**Respuesta `200 OK`:**
```json
{
  "detail": "Credencial eliminada exitosamente"
}
```

#### C. Inicio de Sesión (`POST /auth/login`)
El inicio de sesión exige obligatoriamente identificar la aplicación cliente que solicita credenciales:

##### Opción A: Vía Cabecera HTTP (Recomendada)
```http
POST /auth/login HTTP/1.1
Host: localhost:8000
Content-Type: application/json
X-Client-Id: comensales-app

{
  "email": "usuario@comensales.com",
  "password": "Password123"
}
```

##### Opción B: Vía Campo en el Cuerpo JSON
```http
POST /auth/login HTTP/1.1
Host: localhost:8000
Content-Type: application/json

{
  "email": "usuario@comensales.com",
  "password": "Password123",
  "client_id": "comensales-app"
}
```

> **⚠️ Validación Estricta:** Si ni la cabecera `X-Client-Id` ni el campo `client_id` están presentes, el Gateway responde con `400 Bad Request` (`[E-01]`).

#### Respuesta Exitosa:
```json
{
  "user": "usuario@comensales.com",
  "role": "user",
  "client_id": "comensales-app",
  "token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..."
}
```
El token generado contiene en sus claims estándar:
* `sub`: correo electrónico del usuario.
* `role`: rol global del Gateway (`"admin"` o `"user"`).
* `client_id`: identificador de la app cliente.
* `aud`: audiencia asociada al cliente (`jwt.ClaimStrings{"comensales-app"}`).
* `exp`: timestamp de expiración UTC.


---

### 4. Consumo de Rutas Protegidas y Token Binding
Al consumir cualquier ruta hacia un microservicio aguas abajo (ej. `GET /ventas/items`):

```http
GET /ventas/items HTTP/1.1
Host: localhost:8000
Authorization: Bearer <TOKEN>
X-Client-Id: comensales-app
```

* **Token Binding:** Si la petición incluye la cabecera `X-Client-Id`, el Gateway verifica que coincida estrictamente con el `client_id` sellado dentro del token JWT. Si difieren (intento de suplantación o reenvío cruzado de tokens), responde con `401 Unauthorized` (`[E-02]`).
* **Client-to-Service Partitioning:** El Gateway verifica que el `client_id` del token esté listado en `allowed_clients` de la ruta `/ventas`. Si no lo está, responde `403 Forbidden` (`[E-03]`).

---

### 5. Cabeceras Propagadas hacia los Microservicios Internos

Al validar el JWT en peticiones entrantes hacia los microservicios/backends, el Gateway inyecta de forma fidedigna:

| Cabecera Recibida por el Microservicio | Origen | Descripción |
| :--- | :--- | :--- |
| `X-User-Id` | Gateway (BD/Token) | UUID inmutable del usuario autenticado (`sub` de identidad). |
| `X-User-Email` | Gateway (Token) | Correo electrónico del usuario autenticado. |
| `X-User-Role` | Gateway (Token) | Rol a nivel de sistema del Gateway (`"admin"` o `"user"`). |
| `X-Client-Id` | Gateway (Token) | Identificador de la aplicación cliente que originó la petición. |
| `X-Request-Id` | Gateway (Tracing) | UUID de trazabilidad correlacionada para distributed tracing. |
| `X-Internal-Api-Key` | Gateway (Config) | Secreto interno compartido configurado para el microservicio destino. |
| `X-Scopes` | Gateway (Token) | Scopes o permisos opcionales delegados. |

---

## 📖 Diccionario de Códigos de Error (Error Catalog) & Regla de Fuga Cero

### 🛡️ Regla de Oro: Prohibición Absoluta de Fuga de Información (*Zero-Information Disclosure*)
Siguiendo las mejores prácticas de **OWASP API Security** (API3:2023 Broken Object Property Level Authorization & API8:2023 Security Misconfiguration) y **CWE-209** (*Generation of Error Message Containing Sensitive Information*):

1. **Nunca exponer información estructural al cliente:** Está terminantemente prohibido devolver nombres de parámetros internos, detalles de validaciones de longitud/regex, motivos criptográficos exactos de fallo (ej. diferenciar entre usuario inexistente y contraseña errónea), o trazas de pila (*stack traces*).
2. **Mensajes Opacos y Estandarizados:** Todas las respuestas de error emitidas por el Gateway entregan mensajes genéricos acompañados de un código de catálogo `[E-xx]`.
3. **Trazabilidad Interna Segura (`X-Error-ID`):** Los errores no controlados y anomalías críticas registran la causa técnica detallada exclusivamente en los logs del servidor bajo un identificador único de rastreo correlacionado (`X-Error-ID`), permitiendo que el equipo de soporte depure la incidencia sin entregar vectores de ataque a terceros.

---

### 📋 Catálogo Completo de Errores Perimetrales (`[E-xx]`)

| Código | Código HTTP | Mensaje Retornado al Cliente | Causa Raíz Interna / Guía de Solución para Desarrolladores |
| :---: | :---: | :--- | :--- |
| **`E-01`** | `400 Bad Request` | `"Cabeceras requeridas ausentes o no válidas [E-01]"` | Falta la cabecera obligatoria `X-Client-Id` o el campo `client_id` en el cuerpo JSON del inicio de sesión (`/auth/login`), o contiene caracteres fuera del formato alfanumérico permitido. |
| **`E-02`** | `401 Unauthorized` | `"Contexto de seguridad inconsistente [E-02]"` | **Token Binding Mismatch:** La cabecera `X-Client-Id` enviada en la petición no coincide con el claim `client_id` sellado criptográficamente en el token JWT (intento de reutilización indebida o token robado). |
| **`E-03`** | `403 Forbidden` | `"Acceso al recurso denegado [E-03]"` | **Aislamiento de Clientes (Zero-Trust):** El microservicio destino no cuenta con `allowed_clients` configurado (bloqueo por omisión), el cliente del token no está en su lista blanca, o el rol RBAC del usuario no cumple con `required_roles`. |
| **`E-04`** | `400 Bad Request` | `"Ruta de acceso inválida o no permitida [E-04]"` | **Protección de Enrutamiento:** Detección de secuencias de Path Traversal (`..`, `.`), bytes nulos (`%00`) o caracteres ilegales en la URL solicitada. |
| **`E-05`** | `502 Bad Gateway` | `"Error de comunicación con el servicio de destino [E-05]"` | **Fallo de Conectividad Upstream:** El microservicio interno de destino no está accesible, rechazó la conexión (connection refused) o superó el tiempo límite de espera. |
| **`E-06`** | `404 Not Found` | `"Recurso o servicio no encontrado [E-06]"` | El prefijo solicitado no corresponde a ningún microservicio registrado en `MICROSERVICES`, o la ruta solicitada no existe en el catálogo del Gateway. |
| **`E-07`** | `429 Too Many Requests` | `"Límite de peticiones excedido [E-07]"` | **Rate Limiting Perimetral:** El cliente o la IP de origen ha excedido la cuota de peticiones permitidas por minuto en Redis. |
| **`E-08`** | `401 Unauthorized` | `"No se pudieron validar las credenciales [E-08]"` | **Fallo de Autenticación Universal:** Token JWT ausente, expirado, con firma no válida, o credenciales (usuario/contraseña) incorrectas en `/auth/login` (mitigación de enumeración de cuentas). |
| **`E-09`** | `403 Forbidden` | `"Permisos insuficientes para el recurso [E-09]"` | Intento de consumir un endpoint de gestión administrativa (ej. `POST /users`) con un usuario cuyo rol no es `ADMIN`. |
| **`E-10`** | `422 / 400` | `"Formato de solicitud o cuerpo JSON no válido [E-10]"` | El cuerpo de la petición no contiene un JSON sintácticamente válido o los tipos de datos no corresponden a la estructura esperada. |
| **`E-11`** | `400 Bad Request` | `"Parámetros de solicitud no válidos [E-11]"` | Los campos enviados no superan las validaciones de negocio (ej. email inválido, longitud fuera de rango, etc.). |
| **`E-12`** | `409 Conflict` | `"Conflicto con el recurso solicitado [E-12]"` | Intento de registrar un recurso único que ya existe en el sistema (ej. creación de un usuario con un correo ya registrado). |
| **`E-13`** | `405 Method Not Allowed` | `"Método HTTP no permitido para este recurso [E-13]"` | El verbo HTTP empleado (ej. `DELETE`, `PUT`) no está habilitado para la ruta solicitada en el Gateway. |
| **`E-99`** | `500 Internal Error` | `"Error interno del servidor [E-99]"` | Excepción no controlada o fallo imprevisto. La cabecera HTTP `X-Error-ID` devuelta identifica el evento en los registros internos sin filtrar trazas al exterior. |

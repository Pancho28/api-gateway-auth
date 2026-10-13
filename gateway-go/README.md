# 🚀 API Gateway & Auth Service (Go Implementation)

Servicio de **API Gateway & Identidad Centralizada** migrado a **Go (Golang 1.24)** para máxima eficiencia, latencia sub-milisegundo y optimización de costes en entornos *pay-per-use* (Railway).

---

## 📦 Stack Tecnológico
* **Router & Middlewares:** `github.com/go-chi/chi/v5` + `github.com/go-chi/cors`
* **Reverse Proxy:** `net/http/httputil.ReverseProxy` (Streaming nativo HTTP/1.1 y HTTP/2)
* **Base de Datos:** PostgreSQL 16 con `github.com/jackc/pgx/v5/pgxpool` (Compatible con PgBouncer en Transaction Mode)
* **Caché & Estado:** Redis 7 con `github.com/redis/go-redis/v9`
* **Rate Limiting Distribuido:** `github.com/go-redis/redis_rate/v10`
* **Seguridad & Tokens:** `github.com/golang-jwt/jwt/v5` y hashing PBKDF2 compatible con Passlib (`golang.org/x/crypto/pbkdf2`)

---

## 🛠️ Comandos Principales

### 1. Ejecución Local en Desarrollo
```bash
go run ./cmd/gateway
```

### 2. Ejecutar Suite de Pruebas
```bash
go test -v ./...
```

### 3. Compilación del Binario de Producción
```bash
# Compilar binario nativo
go build -ldflags="-s -w" -o gateway ./cmd/gateway

# O mediante Docker:
docker build -t api-gateway-go .
```

---

## 🛡️ Mejoras Arquitectónicas frente a la Versión Python
1. **Streaming Real Zero-Copy:** `httputil.ReverseProxy` transfiere datos en chunks sin cargarlos íntegramente en la memoria RAM del proceso.
2. **Caché GET Acotado (Protección OOM):** Se limita el cacheo en Redis a payloads de máximo 1 MB. Payloads superiores se transmiten directamente al cliente sin saturar la RAM.
3. **Propagación Segura de Identidad:** Inyección garantizada de cabeceras de contexto (`X-User-ID`, `X-User-Email`, `X-User-Role`, `X-Internal-Api-Key`) hacia los microservicios aguas abajo.
4. **Desacoplamiento de Base de Datos en Proxy:** El middleware valida la sesión en Redis (<0.5 ms). No se solicita ninguna conexión a PostgreSQL a menos que ocurra un *Cache Miss*.
5. **Huella de Memoria Minimizada:** Consumo base en reposo de **~18 MB de RAM** frente a los ~100 MB de Python.

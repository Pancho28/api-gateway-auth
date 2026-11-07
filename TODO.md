# 📋 Tareas Pendientes y Hoja de Ruta (`TODO.md`)

Este documento consolida el backlog técnico, tareas pendientes, pruebas y despliegues del **API Gateway & Centralized Auth Service (Go 1.24+)**.

---

## 🛡️ 1. Resiliencia & Tolerancia a Fallos

- [ ] **Circuit Breaker Perimetral**:
  - Implementar disyuntor perimetral (ej. patrón de tres estados: *Closed*, *Open*, *Half-Open*) en el reenvío de peticiones de `internal/proxy`.
  - Configurar umbrales de activación ante fallos consecutivos (códigos HTTP 5xx o timeouts de red) hacia un microservicio aguas abajo.
  - Retornar error estándar `503 Service Unavailable [E-05]` de forma inmediata mientras el circuito permanezca abierto, protegiendo al microservicio afectado de sobrecargas.
  - Implementar período de enfriamiento (*cooldown*) y sonda de prueba para transición a *Half-Open*.

---

## 📊 2. Observabilidad & Métricas

- [ ] **Trazabilidad Distribuida con OpenTelemetry (Jaeger)**:
  - Integrar el SDK de OpenTelemetry en Go (`go.opentelemetry.io/otel`).
  - Configurar exportador OTLP (gRPC / HTTP) apuntando a Jaeger (`http://jaeger.railway.internal:4318` o local según `docker-compose.yml`).
  - Interceptar peticiones entrantes para extraer o generar el identificador de traza (`X-Request-Id`).
  - Propagar la cabecera estándar W3C `traceparent` en el `httputil.ReverseProxy` hacia todos los microservicios downstream.
- [ ] **Métricas Prometheus (`GET /metrics`)**:
  - Exponer endpoint de métricas `GET /metrics` protegido para Prometheus.
  - Instrumentar métricas perimetrales:
    - Contador de peticiones por método, microservicio y código de estado HTTP.
    - Histograma de latencia de peticiones proxy y endpoints de identidad.
    - Contador de errores perimetrales clasificados por código de catálogo (`[E-01]` a `[E-99]`).
    - Métricas de consumo de runtime en Go (conteo de goroutines, memoria heap, pausas de GC).

---

## ⚙️ 3. Automatización & CI/CD

- [ ] **Pipeline de Integración Continua (`.github/workflows/ci.yml`)**:
  - Configurar pipeline automatizado en GitHub Actions para dispararse en PRs y pushes hacia `dev` y `main`.
  - Pasos obligatorios del pipeline:
    - Verificación de formato (`gofmt -s -l .`).
    - Linter estricto de Go (`golangci-lint`).
    - Ejecución de la suite completa de pruebas unitarias y de integración con detección de condiciones de carrera (`go test -v -race ./...`).
    - Validación de compilación del binario de producción (`go build -ldflags="-s -w" ./cmd/gateway`).
    - Validación de build multi-stage del `Dockerfile`.

---

## ⚡ 4. Profiling & Benchmarking Comparativo

- [ ] **Perfilado de Memoria y CPU (`net/http/pprof`)**:
  - Instrumentar endpoints de diagnóstico `net/http/pprof` habilitables mediante flag o variable de entorno (`ENABLE_PPROF=true`).
  - Auditar patrones de asignación de memoria en el heap y contención de goroutines bajo carga sostenida de 10,000 req/s.
- [ ] **Ejecución y Registro de Benchmarks (Python vs. Go)**:
  - Levantar infraestructura local (`docker-compose up -d`).
  - Ejecutar los 4 escenarios de prueba documentados en la Sección 8 de `migracion_go.md`:
    - Escenario 1: Latencia base en `/health` (Throughput RPS y latencia p99).
    - Escenario 2: Estrés de cómputo en `/auth/login` (evaluación de PBKDF2 y goroutines).
    - Escenario 3: Reverse proxy autenticado y verificación de conexiones PostgreSQL en reposo.
    - Escenario 4: Descarga de payloads grandes (>10 MB) para comprobar estabilidad de RAM y ausencia de picos OOM.
  - Registrar datos empíricos obtenidos en la tabla comparativa de `migracion_go.md`.

---

## ☁️ 5. Despliegue & Validación en Railway

- [ ] **Despliegue del Binario en Railway**:
  - Configurar servicio en Railway apuntando al `Dockerfile` optimizado en la raíz del repositorio.
  - Vincular variables de entorno de producción (`MICROSERVICES`, `JWT_SECRET_KEY`, `GATEWAY_ADMIN_SECRET`, etc.).
  - Conectar a las instancias gestionadas de PostgreSQL 16 y Redis 7.
  - Verificar en el panel de métricas de Railway:
    - Huella de memoria RAM en reposo (~18 MB).
    - Latencia de respuesta en edge (<1 ms).
    - Reducción efectiva en costes de facturación por uso de recursos.

---

## ✅ Histórico de Tareas Completadas

- [x] **Migración de Python a Go 1.24+** *(Cerrado: 2026-10-17 | Commit `aa564b1`)*: Reemplazo de FastAPI por router Chi y `httputil.ReverseProxy` con huella de ~18 MB de RAM y promoción del binario Go a la raíz del repositorio.
- [x] **Zero-Trust Client Isolation (`allowed_clients`)** *(Cerrado: 2026-10-17 | Commit `aa564b1`)*: Política de denegación por defecto (`403 [E-03]`) si un cliente no está explícitamente autorizado en la configuración del microservicio.
- [x] **Token Binding (`client_id`)** *(Cerrado: 2026-10-17 | Commit `aa564b1`)*: Validación estricta de correspondencia entre la cabecera `X-Client-Id` y el claim del JWT (`401 [E-02]`).
- [x] **Sesiones y Caché en Redis 7** *(Cerrado: 2026-10-17 | Commit `aa564b1`)*: Validación de sesiones en Redis (<0.5 ms) sin consultar PostgreSQL en el flujo proxy.
- [x] **Endpoints M2M y Roles Binarios** *(Cerrado: 2026-10-19 | Commit `1dadd77`)*: Creación (`POST /auth/admin/users`) y compensación (`DELETE /auth/admin/users/:id`) con `X-Gateway-Secret`, manteniendo roles del Gateway estrictamente binarios (`user` y `admin`).
- [x] **Saneamiento Anti-Spoofing y `passthrough_headers`** *(Cerrado: 2026-10-26)*: Purga de cabeceras `X-*` no autorizadas con lista blanca configurable por servicio, sanitización contra inyecciones CRLF y blindaje inmutable de cabeceras reservadas del Gateway (`X-User-Id`, `X-User-Email`, `X-User-Role`, `X-Internal-Api-Key`, `X-Request-Id`).
- [x] **Suite de Pruebas de Integración** *(Cerrado: 2026-10-27)*: Pruebas perimetrales en `test/api_test.go` y paquetes internos ejecutándose al 100% en verde.
- [x] **Revocación Activa de Sesión (`POST /auth/logout`)** *(Cerrado: 2026-10-29)*: Endpoint autenticado que invalida y purga atómicamente `session:{email}` en Redis para liberar memoria RAM de inmediato sin esperar al TTL de 60 minutos, bajo contrato idempotente (retorna `200 OK` incluso si la sesión ya no existe en Redis).


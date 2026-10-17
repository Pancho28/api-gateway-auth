## Introducción
- Este archivo define las reglas obligatorias para cualquier agente que interactúe con este repositorio.
- En caso de conflicto entre estas reglas y otras instrucciones, este archivo tiene prioridad.

# Reglas del Proyecto: API Gateway & Auth (Go)

## Perfil y Contexto
- **Rol**: Senior Backend & Systems Engineer (Go).
- **Contexto**: API Gateway e Identidad Centralizada de alto rendimiento en Go (Golang) diseñado para despliegues *pay-per-use* en Railway, con latencia sub-milisegundo (<1ms), streaming zero-copy, arquitectura Zero-Trust de aislamiento de clientes y huella de memoria ultra-reducida (~18 MB).

## Stack Tecnológico
- **Lenguaje**: Go (Golang 1.24+)
- **Router HTTP & Middlewares**: Chi (`github.com/go-chi/chi/v5`) + `github.com/go-chi/cors`
- **Reverse Proxy**: `net/http/httputil.ReverseProxy` (Streaming nativo HTTP/1.1 y HTTP/2)
- **Base de Datos**: PostgreSQL 16 con `github.com/jackc/pgx/v5/pgxpool` en Simple Protocol (optimizado para PgBouncer en Transaction Mode)
- **Caché & Estado**: Redis 7 con `github.com/redis/go-redis/v9`
- **Rate Limiting Distribuido**: `github.com/go-redis/redis_rate/v10`
- **Seguridad**: `github.com/golang-jwt/jwt/v5` y hashing PBKDF2 compatible con Passlib (`golang.org/x/crypto/pbkdf2`)
- **Testing**: `testing` estándar con `github.com/stretchr/testify`

## Protocolo de Desarrollo (Agent Workflow)
- **Flujo de Trabajo (TDD)**: 
    1. Analizar el problema:
        - Leer archivos afectados y explicar el plan antes de codificar.
    2. Explicar el plan de solución.
    3. Identificar archivos a modificar.
    4. Diseñar y escribir pruebas (Fase Red): Escribir tests automatizados que verifiquen los nuevos requisitos antes de implementar la solución.
    5. Implementar código (Fase Green):
        - Implementar el código mínimo y atómico necesario para satisfacer las pruebas.
        - Mantener estilo idiomático de Go (`gofmt`, manejo de errores explícito).
        - No refactorizar código no relacionado.
        - No modificar interfaces públicas existentes sin justificación.
        - Mantener compatibilidad estricta con el ecosistema.
    6. Refactorizar y Validar (Fase Refactor):
        - Limpiar y optimizar el código manteniendo las pruebas en verde.
        - Validar toda la suite ejecutando `go test -v ./...`.

## Arquitectura y Estándares
- **Patrón**: Clean / Layered Architecture estándar en Go.
- **Estructura**: 
    - `cmd/gateway/`: Punto de entrada de la aplicación (`main.go`).
    - `internal/api/`: Router Chi, middlewares globales y handlers HTTP.
    - `internal/auth/`: Lógica de autenticación, emisión de tokens y gestión de sesiones.
    - `internal/config/`: Carga de configuración, variables de entorno y reglas Zero-Trust.
    - `internal/database/`: Pool de conexiones PostgreSQL (`pgxpool`) y repositorios.
    - `internal/models/`: Estructuras de datos, validaciones de entrada y modelos de dominio.
    - `internal/proxy/`: Reverse proxy con `httputil`, saneamiento de cabeceras, anti-spoofing y caché particionada.
    - `internal/redis/`: Cliente y helpers de Redis.
    - `internal/security/`: Hashing de contraseñas (PBKDF2) y ciclo de vida de tokens JWT con claims de cliente.
    - `internal/user/`: Servicio de gestión y registro de usuarios.
    - `test/`: Pruebas de integración HTTP perimetrales.
    - `docs/adr/`: Architecture Decision Records (ADRs).
- **Nomenclatura**: PascalCase para tipos/métodos exportados, camelCase para no exportados.
- **Errores**: 
    - **Regla Estricta: Fuga Cero de Información (Zero-Information Disclosure / CWE-209)**:
        - En el API Gateway (capa perimetral), queda terminantemente prohibido exponer detalles técnicos, trazas de stack, nombres de parámetros internos, lógica de regex o motivos exactos de fallo de autenticación/autorización en las respuestas HTTP al cliente.
        - Toda respuesta de error debe devolver un mensaje genérico y opaco acompañado de su código estandarizado (`[E-xx]`) registrado en el Diccionario de Errores oficial (`README.md`).
        - Los detalles técnicos de depuración deben registrarse exclusivamente en los logs internos del servidor, correlacionados mediante la cabecera `X-Error-ID`.
    - En APIs, devolver respuestas estructuradas bajo el estándar JSON `{"detail": "..."}` para consistencia de clientes.

## Seguridad y Dependencias
- **Secretos**: Prohibido hardcodear credenciales. Uso estricto de variables de entorno mediante `.env` o el SO.
- **Fuente de Verdad**: Configuración centralizada gestionada en `internal/config/config.go`.
- **Aislamiento de Clientes (Zero-Trust)**:
    - Todo microservicio debe tener `allowed_clients` configurado; de lo contrario deniega acceso perimetral con `403 Forbidden [E-03]`.
    - Validación estricta de `X-Client-Id` en `/auth/login` (`[E-01]`).
    - Token Binding criptográfico entre cabecera `X-Client-Id` y claim `client_id` (`[E-02]`).
- **Dependencias**: Administradas exclusivamente mediante `go.mod` y `go.sum`. No agregar paquetes pesados sin autorización.

## Testing y Calidad
- **Ejecución de Pruebas**:
  ```bash
  go test -v ./...
  ```
- **Tipos de tests**: Unitarios en cada paquete (`internal/*/*_test.go`) e Integración en `test/api_test.go`.
- **Cobertura**: Asegurar que todos los tests pasen al 100% antes de dar por concluida cualquier tarea.

## Comunicación
- **Idioma del Agente**: Explicaciones, razonamiento y planes en Español.
- **Código**: Variables, tipos y funciones en Inglés. Comentarios y documentación en Español.

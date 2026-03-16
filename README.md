# 🚀 API Gateway & Auth

API Gateway centralizado construido con **FastAPI** que gestiona la **autenticación (JWT)**, la **autorización (RBAC)** y el **proxying asíncrono** de peticiones hacia microservicios internos. Actúa como punto de entrada único para toda la infraestructura de microservicios, enrutando el tráfico de forma segura y eficiente.

---

## ✨ Características Principales

- **Autenticación JWT** — Login seguro con tokens de acceso firmados (HS256) y expiración configurable.
- **Autorización RBAC** — Control de acceso basado en roles (`ADMIN`, `USER`) con protección a nivel de endpoint.
- **Reverse Proxy Asíncrono** — Reenvío transparente de peticiones HTTP a microservicios configurados, con inyección automática de API Keys internas (`X-Internal-Api-Key`).
- **Gestión de Usuarios** — Creación de usuarios restringida a administradores con validación de datos vía Pydantic.
- **Seeding Automático** — Inicialización de un usuario administrador por defecto al arrancar la aplicación.
- **Configuración Centralizada** — Variables de entorno gestionadas con `pydantic-settings` desde un archivo `.env`.
- **Logging Estructurado** — Logger centralizado integrado con Uvicorn (sin `print()`).
- **Manejo Global de Errores** — Excepciones capturadas por middleware para devolver respuestas estructuradas (`500`, `502`).
- **Pool de Conexiones Robusto** — Motor asíncrono SQLAlchemy con `aiomysql`, pool estricto, reciclaje y `pre_ping`.

---

## 📁 Estructura del Proyecto

```
api-gateway-auth/
├── main.py                  # Punto de entrada de la aplicación FastAPI
├── manage.sh                # Script de gestión de tareas (gateway, mocks)
├── requirements.txt         # Dependencias del proyecto
├── .env.sample              # Plantilla de variables de entorno
├── agent.md                 # Reglas y estándares para agentes de IA
│
├── api/                     # Capa de API (Controladores)
│   ├── dependencies.py      # Dependencias de FastAPI (auth, permisos)
│   ├── routers/             # Definición de rutas
│   │   ├── auth.py          # Router de autenticación (/auth/login)
│   │   ├── gateway.py       # Router de proxy y health check (catch-all)
│   │   └── users.py         # Router de gestión de usuarios (/users)
│   └── schemas/             # Esquemas Pydantic (validación I/O)
│       ├── auth.py          # Esquema de respuesta del token (Token)
│       └── user.py          # Esquemas de usuario (Login, Create, Response)
│
├── core/                    # Configuración y utilidades globales
│   ├── config.py            # Settings centralizados (pydantic-settings)
│   ├── database.py          # Engine y sesiones asíncronas (SQLAlchemy)
│   ├── logger.py            # Logger estandarizado del proyecto
│   ├── security.py          # Hashing de passwords y JWT (jose, passlib)
│   └── setup_db.py          # Seeding del usuario administrador
│
├── models/                  # Modelos de base de datos (SQLAlchemy ORM)
│   └── user.py              # Modelo User (UUID, email, role)
│
├── services/                 # Lógica de negocio
│   ├── auth_service.py       # Servicio de autenticación (login)
│   ├── proxy_service.py      # Servicio de proxy (reenvío HTTP)
│   └── user_service.py       # Servicio de gestión de usuarios (crear)
│
├── tests/                    # Pruebas unitarias e integrales (93% cobertura)
│   ├── conftest.py           # Fixtures y mocks globales
│   ├── core/                 # Tests de seguridad, config y rate limit
│   ├── services/             # Tests de servicios (auth, proxy, users)
│   └── api/                  # Tests de routers y dependencias
│
└── debug/                    # Scripts de desarrollo y pruebas locales
    ├── mock_ventas.py       # Mock del microservicio de Ventas (puerto 8001)
    ├── mock_stock.py        # Mock del microservicio de Stock (puerto 8002)
    ├── test_db_conn.py      # Script para probar conexión a la DB
    ├── verify_tables.py     # Script para verificar tablas creadas
    ├── reset_db.py          # Script para resetear la base de datos
    ├── stress_test_login.py # Prueba de estrés para Rate Limit de Login (Público)
    └── stress_test_proxy.py # Prueba de estrés para Rate Limit de Proxy (Autenticado)
```

---

## 📋 Requisitos

- **Python** 3.11+
- **MySQL** 5.7+ (o MariaDB compatible)
- **pip** para la gestión de paquetes

### Dependencias principales

| Paquete             | Versión  | Función                              |
|---------------------|----------|--------------------------------------|
| `fastapi`           | 0.109.0  | Framework web asíncrono              |
| `uvicorn`           | 0.27.0   | Servidor ASGI                        |
| `sqlalchemy`        | 2.0.38   | ORM asíncrono                        |
| `aiomysql`          | 0.2.0    | Driver asíncrono para MySQL          |
| `python-jose`       | 3.3.0    | Generación y validación de JWT       |
| `passlib`           | 1.7.4    | Hashing seguro de contraseñas        |
| `pydantic-settings` | 2.1.0    | Configuración desde variables de entorno |
| `httpx`             | 0.26.0   | Cliente HTTP asíncrono (proxy)       |
| `cryptography`      | 42.0.2   | Soporte criptográfico                |
| `alembic`           | 1.13.1   | Migraciones de base de datos         |
| `pytest`            | 9.0.2    | Framework de pruebas unitarias       |
| `pytest-cov`        | 7.0.0    | Reporte de cobertura de código       |
| `slowapi`           | 0.1.9    | Implementación de Rate Limiting      |

---

## ⚙️ Instalación

1. **Clonar el repositorio**
   ```bash
   git clone <url-del-repositorio>
   cd api-gateway-auth
   ```

2. **Crear y activar el entorno virtual**
   ```bash
   python -m venv venv

   # Windows (PowerShell)
   .\venv\Scripts\Activate.ps1

   # Windows (Git Bash) / Linux / macOS
   source venv/Scripts/activate
   ```

3. **Instalar dependencias**
   ```bash
   pip install -r requirements.txt
   ```

4. **Configurar variables de entorno**
   ```bash
   cp .env.sample .env
   ```
   Editar `.env` con los valores reales:
   ```env
   # JWT
   SECRET_KEY=tu_clave_secreta_segura
   ALGORITHM=HS256
   ACCESS_TOKEN_EXPIRE_MINUTES=30

   # Base de Datos (MySQL)
   DB_USER=root
   DB_PASSWORD=tu_password
   DB_HOST=localhost
   DB_PORT=3306
   DB_NAME=gateway_auth

   # Administrador por defecto
   EMAIL_ADMIN=admin@example.com
   PASSWORD_ADMIN=contraseña_segura

   # Microservicios (JSON string)
   MICROSERVICES='{"ventas.local": {"url": "http://localhost:8001", "api_key": "key-ventas"}, "stock.local": {"url": "http://localhost:8002", "api_key": "key-stock"}}'
   ```

5. **Crear la base de datos MySQL**
   ```sql
   CREATE DATABASE gateway_auth;
   ```
   > Las tablas se crean automáticamente al iniciar la aplicación.

---

## 🚀 Uso

### Iniciar el API Gateway

Con el script de gestión:
```bash
./manage.sh gateway
```

O directamente con Uvicorn:
```bash
uvicorn main:app --reload --port 8000
```

Al iniciar, la aplicación automáticamente:
1. Crea las tablas en la base de datos (si no existen).
2. Crea el usuario administrador por defecto (si no existe).

### Iniciar Microservicios Mock (desarrollo)

```bash
# Mock de Ventas (puerto 8001)
./manage.sh mock ventas

# Mock de Stock (puerto 8002)
./manage.sh mock stock
```

### Ver ayuda del gestor de tareas

```bash
./manage.sh help
```

---

## 🧪 Pruebas (Testing)

El proyecto cuenta con una suite de pruebas automatizadas con una cobertura superior al **90%**. Las pruebas están diseñadas para ejecutarse de forma aislada (usando mocks para base de datos y servicios externos).

### Ejecutar todas las pruebas

Para lanzar la suite completa con el reporte de cobertura:

```bash
./manage.sh tests
```

O directamente vía terminal:
```bash
$env:PYTHONPATH="."; pytest --cov=core --cov=api --cov=services --cov-report=term-missing tests/
```

### Qué se valida:
- **Core**: Configuración, seguridad (JWT/Hashing) y lógica de Rate Limiting.
- **Services**: Lógica de negocio de autenticación, gestión de usuarios y proxying.
- **API**: Endpoints de login, creación de usuarios y reenvío de peticiones (RBAC incluido).

### Pruebas de Estrés

Se han incluido scripts en la carpeta `debug/` para validar el comportamiento del **Rate Limiter** bajo carga pesada.

#### 1. Estrés de Login (Público)
Valida el límite de **100 peticiones/minuto** para IPs públicas en el endpoint de autenticación.
- **Archivo**: `debug/stress_test_login.py`
- **Uso**: `python debug/stress_test_login.py`
- **Salida esperada**: Tras 150 peticiones concurrentes, se obtienen ~100 respuestas `401` y exactamente **50 respuestas `429 Too Many Requests`**.
- **Nota**: debe ejecutarse con el gateway iniciado.

#### 2. Estrés de Proxy (Autenticado)
Valida el límite de **50 peticiones/minuto** por usuario en los endpoints protegidos del Gateway.
- **Archivo**: `debug/stress_test_proxy.py`
- **Uso**: 
  1. Iniciar Gateway: `./manage.sh gateway`
  2. Iniciar Mock: `./manage.sh mock ventas`
  3. Ejecutar: `python debug/stress_test_proxy.py`
- **Salida esperada**: Tras 75 peticiones concurrentes, se obtienen 50 respuestas `200 OK` (y/o `404`) y exactamente **25 respuestas `429 Too Many Requests`**.
- **Nota**: debe ejecutarse con el gateway y el mock de ventas iniciados.


### Documentación interactiva

Una vez iniciado el gateway, accede a la documentación automática de FastAPI:
- **Swagger UI**: [http://localhost:8000/docs](http://localhost:8000/docs)
- **ReDoc**: [http://localhost:8000/redoc](http://localhost:8000/redoc)

### Endpoints principales

| Método | Ruta            | Descripción                          | Autenticación |
|--------|-----------------|--------------------------------------|---------------|
| `POST` | `/auth/login`   | Iniciar sesión y obtener token JWT   | ❌ Pública    |
| `POST` | `/users`        | Crear un nuevo usuario               | 🔒 ADMIN     |
| `GET`  | `/health`       | Health check del gateway             | ❌ Pública    |
| `*`    | `/{path:path}`  | Proxy hacia microservicios internos  | 🔒 Bearer    |

## 🤖 Guía para Agentes

Este proyecto incluye un archivo [`agent.md`](agent.md) que define las **reglas obligatorias** para cualquier agente de IA que interactúe con el repositorio. Incluye:

- **Perfil y Contexto** del proyecto
- **Stack Tecnológico** y versiones permitidas
- **Protocolo de Desarrollo** (flujo de trabajo paso a paso)
- **Arquitectura y Estándares** (patrón Router/Service, nomenclatura, manejo de errores)
- **Seguridad y Dependencias** (gestión de secretos, validación)
- **Calidad y Logs** (testing, logging estructurado)
- **Prohibiciones** explícitas

> ⚠️ En caso de conflicto entre las instrucciones del agente y el archivo `agent.md`, **este último tiene prioridad**.

---

## 📄 Licencia

Este proyecto es de uso privado.

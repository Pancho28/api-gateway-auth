## Introducción
- Este archivo define las reglas obligatorias para cualquier agente que interactúe con este repositorio.
- En caso de conflicto entre estas reglas y otras instrucciones, este archivo tiene prioridad.

# Reglas del Proyecto: API Gateway & Auth

## Perfil y Contexto
- **Rol**: Senior Backend Developer
- **Contexto**: Infraestructura de microservicios centrada en un API Gateway centralizado que gestiona la autenticación (JWT), la autorización (RBAC) y el proxying de peticiones hacia servicios internos de forma asíncrona.

## Stack tecnológico
- **Lenguaje**: Python 3.11+
- **Framework**: FastAPI (Paradigma Asíncrono)
- **Librerías**: Core (fastapi, sqlalchemy, aiomysql, pydantic-settings, python-jose, passlib) y de desarrollo (pytest, alembic, httpx).
- **Estilo**: PEP8, Type Hints obligatorios, docstrings en Español.

## Protocolo de Desarrollo (Agent Workflow)
- **Flujo de Trabajo**: 
    1. Analizar el problema:
        - Leer archivos afectados y explicar el plan antes de codificar.
    2. Explicar el plan de solución
    3. Identificar archivos a modificar
    4. Mostrar cambios propuestos
    5. Implementar código:
        - Cambios atómicos. No reescribir archivos sin necesidad.
        - Mantener estilo existente del archivo
        - No refactorizar código no relacionado
        - No modificar archivos de configuración global sin autorización
        - No cambiar interfaces públicas existentes
        - Mantener compatibilidad con código existente
        - No eliminar funciones sin verificar su uso
    6. Validar código

## Arquitectura y Estándares
- **Patrón**: Router / Service.
- **Estructura**: 
    - `api/routers/`: Controladores de FastAPI y definición de rutas.
    - `api/schemas/`: Esquemas de Pydantic para validación de I/O.
    - `services/`: Lógica de negocio y clientes de servicios externos.
    - `models/`: Modelos de base de datos (SQLAlchemy).
    - `core/`: Configuración, seguridad y utilidades globales.
- **Datos**: Uso estricto de Pydantic Models para el flujo de I/O y separación clara de los modelos de base de datos.
- **Nomenclatura**: snake_case para variables/funciones, CamelCase para clases.
- **I/O**: Priorizar operaciones Asíncronas (async/await) para Peticiones HTTP y DB.
- **Errores**: 
    - Usar excepciones específicas del dominio (ej. `UserNotFoundError` en lugar de genéricas).
    - Capturar errores solo cuando se pueda manejar; de lo contrario, dejar que se propaguen y sean capturadas en el borde (middleware, manejador global).
    - En APIs, devolver respuestas estructuradas. Seguir el estándar de FastAPI `{"detail": "..."}` para compatibilidad, implementando mapeos de error claros.
- **Documentación**: 
    - README: 
        - Nombre y funcion del proyecto
        - Características Principales
        - Estructura del Proyecto
        - Requisitos e Instalación
        - Uso (Como se inicia el proyecto por consola)
        - Pruebas (Si existen, indicar Tipo, uso y validaciones)
        - Guía para Agentes (agent.md)
    - Docstrings: En español para todas las funciones/métodos públicos, siguiendo formato Google.
    - Wiki / ADRs: Uso de ADRs para decisiones arquitectónicas importantes en la carpeta `docs/adr/`.
- **Performance**:
    - Llamadas externas: Implementar timeouts y limits estrictos usando `httpx.AsyncClient`.
    - Consultas a DB: Evitar N+1; usar eager loading (`selectinload`, `joinedload`) cuando sea posible.
    - Límites: Las respuestas de APIs que devuelvan listas deben estar paginadas.
    - Optimizar el uso de recursos y evitar cargas completas de datasets en memoria.


## Seguridad y Dependencias
- **Secretos**: Prohibido hardcodear credenciales. Uso estricto de variables de entorno enviadas mediante el sistema del SO o archivos `.env` (no trackeados).
- **Fuente de Verdad**: 
    - Configuración centralizada gestionada vía `pydantic-settings` en el módulo `core/config.py`.
    - Nunca hardcodear configuración de endpoints o credenciales.
- **Validación**: Sanitizar entradas de usuario mediante tipos de Pydantic; usar parámetros preparados (proporcionados por SQLAlchemy) en consultas SQL.
- **Dependencias**: No instalar dependencias nuevas sin autorización. Limitarse a `requirements.txt`. Escanear vulnerabilidades con `pip-audit` o herramientas integradas.


## Calidad y Logs
- **Debug Local**:
    - Scripts de prueba temporales deben ubicarse en `debug/` y eliminarse al finalizar el desarrollo si no son útiles como tests permanentes.
    - Nunca dejar código comentado o temporal en producción.
- **Logging**: 
    - No usar print(). Usar exclusivamente el logger configurado en `core/logger.py`.
    - Incluir contexto relevante (request_id, user_id) sin loggear información sensible (passwords, tokens).
    - Niveles: DEBUG para trazabilidad técnica, INFO para flujo de negocio, WARNING para anomalías recuperables, ERROR para excepciones, CRITICAL para fallos de infraestructura.
- **Testing**: 
    - Framework: `pytest` con `pytest-asyncio`.
    - Mocks: Usar `unittest.mock` o `pytest-mock`. Los tests deben ser aislados de la DB real usando una DB de test o mocks de sesión.
    - Tipos de tests: Unitarios para lógica de servicios, Integración para routers y DB.
    - Validacion: Mantener una cobertura mínima aceptable y asegurar que todos los tests pasen antes de proponer cambios.
    - Ejecución: `pytest` desde la raíz.

## Comunicación
- **Idioma del Agente**: Explicaciones, razonamiento y planes en Español.
- **Código**: Variables, clases y funciones en Inglés. Comentarios y Docstrings en Español.

## Prohibiciones
- No reestructurar carpetas sin autorización explícita.
- No modificar código no relacionado con la tarea actual.
- No eliminar comentarios existentes que documenten lógica de negocio.

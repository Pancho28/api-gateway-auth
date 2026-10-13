# ADR-001: Migración de MySQL a PostgreSQL con asyncpg y Connection Pooling mediante PgBouncer

* **Estado:** Aceptado
* **Fecha:** Octubre 2026
* **Decisores:** Senior Backend Architecture Team
* **Contexto Técnico:** API Gateway & Auth Infrastructure

---

## 1. Contexto y Problema

El API Gateway actúa como el punto único de entrada para el ecosistema de microservicios. Inicialmente, la capa de persistencia fue concebida sobre MySQL/MariaDB utilizando `aiomysql`. 

Conforme se proyecta el crecimiento del sistema y el escalado horizontal (múltiples réplicas/pods del API Gateway):
1. **Agotamiento de Conexiones (*Connection Exhaustion*):** Cada instancia de FastAPI gestiona su propio pool de conexiones en memoria. Al escalar a múltiples pods, el número acumulado de conexiones simultáneas supera los límites del sistema operativo y del motor de base de datos, degradando el rendimiento o provocando fallos por rechazo de conexiones.
2. **Capacidades de Concurrencia y Datos:** PostgreSQL ofrece un modelo de concurrencia MVCC superior, soporte nativo de tipos de datos JSON binarios (`JSONB`), índices GIN/GiST, y soporte para tipos de identificadores nativos (`UUID`), esenciales para las siguientes fases de la plataforma.

---

## 2. Decisión

Se decide migrar la capa de persistencia de MySQL a **PostgreSQL** complementado con **PgBouncer** como Connection Pooler intermedio:

1. **Driver Asíncrono:** Adoptar **`asyncpg`** en conjunto con SQLAlchemy 2.0 (`postgresql+asyncpg://`), aprovechando que es el driver asíncrono para PostgreSQL más rápido en el ecosistema Python.
2. **Connection Pooling con PgBouncer:**
   - Desplegar PgBouncer delante de PostgreSQL en modo de agrupación por transacción (`pool_mode = transaction`).
   - El API Gateway se conectará al puerto expuesto por PgBouncer (`6432`), multiplexando cientos de peticiones de clientes sobre un conjunto reducido y persistente de conexiones reales hacia PostgreSQL.
3. **Manejo de Sentencias Preparadas (*Prepared Statements*):**
   - Debido a que en `pool_mode = transaction` PgBouncer reasigna las conexiones físicas del backend entre transacciones, las sentencias preparadas cacheadas en memoria de la conexión física pueden colisionar o no encontrarse (`prepared statement does not exist`).
   - **Regla estricta:** Se debe configurar el motor de SQLAlchemy con `statement_cache_size = 0` y `prepared_statement_cache_size = 0` en `connect_args`.

---

## 3. Consecuencias

### Positivas
* **Escalabilidad Horizontal Segura:** Múltiples instancias del API Gateway pueden conectarse concurrentemente sin riesgo de saturar el límite de conexiones de PostgreSQL.
* **Rendimiento:** `asyncpg` provee una serialización binaria ultrarrápida y mínimo consumo de CPU.
* **Preparación para Futuras Fases:** Capacidad de almacenamiento estructurado y semi-estructurado óptimo, con base sólida para la integración de Redis en la Fase 3.

### Trade-offs / Mitigaciones
* **Servicio Adicional en Infraestructura:** Se requiere el despliegue y mantenimiento del contenedor de PgBouncer en `docker-compose.yml` y entornos de producción.
* **Prepared Statements:** Se desactiva el cacheo de sentencias preparadas en el cliente para mantener compatibilidad total con el modo de transacción de PgBouncer.

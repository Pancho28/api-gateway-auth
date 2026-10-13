import pytest
from core.database import engine

def test_engine_postgresql_asyncpg_configuration():
    """Valida que el engine de SQLAlchemy use PostgreSQL con asyncpg."""
    assert engine.dialect.name == "postgresql"
    assert engine.dialect.driver == "asyncpg"

def test_engine_pgbouncer_prepared_statement_mitigation():
    """
    Valida que el engine desactive la caché de prepared statements en asyncpg
    para compatibilidad estricta con PgBouncer en transaction mode (ADR-001).
    """
    connect_args = engine.url.query.get("statement_cache_size") or engine.dialect.connect_args.get("statement_cache_size") if hasattr(engine.dialect, "connect_args") else None
    # Verificamos que connect_args contenga statement_cache_size=0
    # En create_async_engine(..., connect_args={"statement_cache_size": 0})
    raw_connect_args = getattr(engine.sync_engine.pool, "_creator", None)
    # También podemos inspeccionar engine.sync_engine.url o los connect_args del dialecto
    # O validar que el dialecto / connect_args configurados no permitan prepared statement cache
    from core.database import ASYNC_CONNECT_ARGS
    assert ASYNC_CONNECT_ARGS.get("statement_cache_size") == 0
    assert ASYNC_CONNECT_ARGS.get("prepared_statement_cache_size") == 0

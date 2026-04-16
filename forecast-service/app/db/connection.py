from functools import lru_cache
from sqlalchemy.ext.asyncio import AsyncEngine, create_async_engine


@lru_cache(maxsize=1)
def get_engine(database_url: str) -> AsyncEngine:
    """Cached async SQLAlchemy engine. READ only — never write to Person 1's ledger."""
    return create_async_engine(
        database_url,
        pool_size=5,
        max_overflow=2,
        pool_pre_ping=True,
        echo=False,
    )

from collections.abc import AsyncIterator
from contextlib import asynccontextmanager
from dataclasses import dataclass

from langgraph.checkpoint.postgres.aio import AsyncPostgresSaver
from psycopg_pool import AsyncConnectionPool
from redis.asyncio import Redis

from app.settings import settings


@dataclass
class Container:
    pg_pool: AsyncConnectionPool | None = None
    checkpointer: AsyncPostgresSaver | None = None
    redis: Redis | None = None


container: Container = Container()


@asynccontextmanager
async def lifespan_deps() -> AsyncIterator[None]:
    # autocommit=True avoids long-running transactions holding locks during LLM calls.
    # prepare_threshold=0 is safe behind a pooler like PgBouncer.
    pool: AsyncConnectionPool = AsyncConnectionPool(
        conninfo=settings.database_url,
        max_size=20,
        kwargs={"autocommit": True, "prepare_threshold": 0},
        open=False,
    )
    await pool.open()
    container.pg_pool = pool

    checkpointer = AsyncPostgresSaver(pool)
    await checkpointer.setup()
    container.checkpointer = checkpointer

    redis_client: Redis = Redis.from_url(settings.redis_url, decode_responses=True)
    await redis_client.ping()
    container.redis = redis_client

    try:
        yield
    finally:
        if container.redis is not None:
            await container.redis.aclose()
            container.redis = None
        if container.pg_pool is not None:
            await container.pg_pool.close()
            container.pg_pool = None
        container.checkpointer = None

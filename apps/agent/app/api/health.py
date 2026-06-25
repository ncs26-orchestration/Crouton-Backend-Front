from typing import Literal, TypedDict

from fastapi import APIRouter

from app.deps import container

router = APIRouter(tags=["health"])


class HealthResponse(TypedDict):
    status: Literal["ok"]


class ReadyResponse(TypedDict):
    status: Literal["ok", "degraded"]
    db: Literal["up", "down"]
    redis: Literal["up", "down"]


@router.get("/healthz")
async def healthz() -> HealthResponse:
    return {"status": "ok"}


@router.get("/readyz")
async def readyz() -> ReadyResponse:
    db: Literal["up", "down"] = "up"
    redis_state: Literal["up", "down"] = "up"

    if container.pg_pool is None:
        db = "down"
    else:
        try:
            async with container.pg_pool.connection() as conn:
                await conn.execute("SELECT 1")
        except Exception:
            db = "down"

    if container.redis is None:
        redis_state = "down"
    else:
        try:
            await container.redis.ping()
        except Exception:
            redis_state = "down"

    status: Literal["ok", "degraded"] = (
        "ok" if db == "up" and redis_state == "up" else "degraded"
    )
    return {"status": status, "db": db, "redis": redis_state}

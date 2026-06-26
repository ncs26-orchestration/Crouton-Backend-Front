from collections.abc import AsyncGenerator
from contextlib import asynccontextmanager

from fastapi import FastAPI
from fastapi.middleware.cors import CORSMiddleware

from app.api import attachments, chat, copilot, extract, health, interview
from app.deps import lifespan_deps


@asynccontextmanager
async def lifespan(_: FastAPI) -> AsyncGenerator[None]:
    async with lifespan_deps():
        yield


def create_app() -> FastAPI:
    app = FastAPI(title="aios-agent", version="0.0.0", lifespan=lifespan)
    app.add_middleware(
        CORSMiddleware,
        allow_origins=["*"],
        allow_methods=["*"],
        allow_headers=["*"],
    )
    app.include_router(health.router)
    app.include_router(chat.router)
    app.include_router(extract.router)
    app.include_router(copilot.router)
    app.include_router(attachments.router)
    app.include_router(interview.router)
    return app


app = create_app()

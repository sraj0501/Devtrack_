"""
Admin Console FastAPI application — runs on ADMIN_PORT (default 8090).

Entry points:
    python -m backend.admin
    uvicorn backend.admin.app:app --port 8090
"""
from __future__ import annotations

import os
from contextlib import asynccontextmanager
from pathlib import Path

from fastapi import FastAPI
from fastapi.staticfiles import StaticFiles

from backend.admin.routes import router, startup


@asynccontextmanager
async def lifespan(app: FastAPI):
    startup()
    yield


app = FastAPI(
    title="DevTrack Admin Console",
    docs_url=None,    # no Swagger UI for admin — keep the attack surface small
    redoc_url=None,
    lifespan=lifespan,
)

# Mount static files
_STATIC_DIR = Path(__file__).parent / "static"
app.mount("/admin/static", StaticFiles(directory=str(_STATIC_DIR)), name="admin-static")

# Mount all admin routes under /admin
app.include_router(router, prefix="/admin")


def main() -> None:
    import uvicorn
    from backend.config import get_admin_port, get_admin_host
    port = get_admin_port()
    host = get_admin_host()
    uvicorn.run("backend.admin.app:app", host=host, port=port, reload=False)

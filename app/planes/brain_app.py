from __future__ import annotations

from dotenv import load_dotenv

# Load env FIRST (before importing modules that read env vars)
load_dotenv()

from fastapi import FastAPI  # noqa: E402

from app.core.logging_config import setup_logging  # noqa: E402
from app.core.sentry import setup_sentry  # noqa: E402
from app.core.otel import setup_otel  # noqa: E402
from app.middleware.request_context import RequestContextMiddleware  # noqa: E402

from app.api.routes.health import router as health_router  # noqa: E402
from app.api.internal.brain import router as brain_router  # noqa: E402
from app.api.internal.llm import router as llm_router  # noqa: E402

setup_logging()
setup_sentry()

app = FastAPI(title="Executive OS — Brain", version="1.0.0")
app.add_middleware(RequestContextMiddleware)

setup_otel(app)

app.include_router(health_router)
app.include_router(brain_router)
app.include_router(llm_router)

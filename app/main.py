# app/main.py
from __future__ import annotations

from dotenv import load_dotenv

# Load env FIRST (before importing modules that read env vars)
load_dotenv()

from contextlib import asynccontextmanager

from fastapi import FastAPI, Request
from fastapi.middleware.cors import CORSMiddleware
from fastapi.responses import JSONResponse

from app.core.config import settings  # noqa: E402
from app.core.logging_config import setup_logging  # noqa: E402
from app.core.sentry import setup_sentry  # noqa: E402
from app.core.metrics import MetricsMiddleware  # noqa: E402
from app.core.otel import setup_otel  # noqa: E402
from app.middleware.request_context import RequestContextMiddleware  # noqa: E402

# Configure structured logging before anything else
setup_logging()
setup_sentry()

# Import DB AFTER env is loaded so DATABASE_URL is correct
from app.db.database import Base, engine  # noqa: E402
import app.db.models  # noqa: F401, E402

# Rate limiting
from app.middleware.rate_limiter import limiter, setup_rate_limiting  # noqa: E402
from slowapi.errors import RateLimitExceeded  # noqa: E402
from app.middleware.auth import AuthMiddleware  # noqa: E402

from app.api.routes.health import router as health_router  # noqa: E402
from app.api.routes.metrics import router as metrics_router  # noqa: E402
from app.api.routes.chat import router as chat_router  # noqa: E402
from app.api.routes.device import router as device_router  # noqa: E402
from app.api.routes.watch import router as watch_router  # noqa: E402
from app.api.routes.discover import router as discover_router  # noqa: E402
from app.api.routes.assist import router as assist_router  # noqa: E402
from app.api.routes.watch_refresh import router as watch_refresh_router  # noqa: E402
from app.api.routes.notifications import router as notifications_router  # noqa: E402
from app.api.routes.agent_chat import router as agent_chat_router  # noqa: E402
from app.api.routes import webhooks_whatsapp  # noqa: E402
from app.api.routes.admin_google import router as admin_google_router  # noqa: E402
from app.api.routes.admin_microsoft import router as admin_microsoft_router  # noqa: E402
from app.api.routes.admin_caldav import router as admin_caldav_router  # noqa: E402
from app.api.routes.admin_tasks import router as admin_tasks_router  # noqa: E402
from app.api.routes.billing import router as billing_router  # noqa: E402
from app.api.routes.billing_stripe import router as billing_stripe_router  # noqa: E402
from app.api.routes.proposals import router as proposals_router  # noqa: E402
from app.api.routes.monitoring import router as monitoring_router  # noqa: E402
from app.api.routes.payment import router as payment_router  # noqa: E402
from app.api.routes.execution import router as execution_router  # noqa: E402
from app.api.routes.webhooks import router as webhooks_router  # noqa: E402
from app.api.routes.intervention import router as intervention_router  # noqa: E402
from app.api.routes.dashboard import router as dashboard_router  # noqa: E402
from app.api.routes.travel import router as travel_router  # noqa: E402
from app.api.routes.gdpr import router as gdpr_router  # noqa: E402
from app.api.routes.bookings import router as bookings_router  # noqa: E402
from app.api.routes.voice import router as voice_router  # noqa: E402
from app.api.routes.voice import webhook_router as voice_webhook_router  # noqa: E402
from app.api.routes.files import router as files_router  # noqa: E402
from app.api.routes.photos import router as photos_router  # noqa: E402
from app.api.routes.profile import router as profile_router  # noqa: E402
from app.api.routes.consent import router as consent_router  # noqa: E402
from app.api.routes.onboarding import router as onboarding_router  # noqa: E402
from app.api.routes.proactive import router as proactive_router  # noqa: E402
from app.api.routes.smart_home import router as smart_home_router  # noqa: E402
from app.api.routes.admin_smart_home import router as admin_smart_home_router  # noqa: E402


_scheduler = None


@asynccontextmanager
async def lifespan(app: FastAPI):
    global _scheduler

    if settings.ENABLE_CREATE_ALL == "1":
        Base.metadata.create_all(bind=engine)

    if settings.ENABLE_SCHEDULER == "1":
        from app.services.scheduler import start_scheduler
        _scheduler = start_scheduler()

    yield

    if _scheduler:
        from app.services.scheduler import stop_scheduler
        stop_scheduler(_scheduler)


app = FastAPI(
    title="Executive AI Agent",
    version="1.0.0",
    lifespan=lifespan,
)

# --- Authentication ---
app.add_middleware(AuthMiddleware)
app.add_middleware(RequestContextMiddleware)
if settings.PROMETHEUS_ENABLED:
    app.add_middleware(MetricsMiddleware)

# --- Optional CORS (safe defaults; tighten later) ---
origins = [o.strip() for o in settings.CORS_ORIGINS.split(",") if o.strip()]
if origins:
    app.add_middleware(
        CORSMiddleware,
        allow_origins=origins,
        allow_credentials=True,
        allow_methods=["*"],
        allow_headers=["*"],
    )

# --- Rate Limiting ---
# Add rate limiter to app state
app.state.limiter = limiter

# Custom rate limit exceeded handler
@app.exception_handler(RateLimitExceeded)
async def rate_limit_handler(request: Request, exc: RateLimitExceeded):
    return JSONResponse(
        status_code=429,
        content={
            "error": "rate_limit_exceeded",
            "message": "Too many requests. Please slow down.",
            "retry_after": exc.detail,
        },
    )

# Setup rate limiting middleware
setup_rate_limiting(app)

# Routers
app.include_router(health_router, tags=["health"])
if settings.PROMETHEUS_ENABLED:
    app.include_router(metrics_router)
app.include_router(chat_router, prefix="/chat", tags=["chat"])
app.include_router(device_router, prefix="/device", tags=["device"])
app.include_router(watch_router, prefix="/watch", tags=["watch"])
app.include_router(discover_router, prefix="/discover", tags=["discover"])
app.include_router(assist_router, prefix="/assist", tags=["assist"])
app.include_router(watch_refresh_router)
app.include_router(notifications_router)
app.include_router(agent_chat_router)
app.include_router(webhooks_whatsapp.router)
app.include_router(admin_google_router)
app.include_router(admin_microsoft_router)
app.include_router(admin_caldav_router)
app.include_router(admin_tasks_router)
app.include_router(billing_router)
app.include_router(billing_stripe_router)
app.include_router(proposals_router)
app.include_router(monitoring_router)
app.include_router(payment_router)
app.include_router(execution_router)
app.include_router(webhooks_router)
app.include_router(intervention_router)
app.include_router(dashboard_router)
app.include_router(travel_router)
app.include_router(gdpr_router)
app.include_router(bookings_router)
app.include_router(voice_router)
app.include_router(voice_webhook_router)
app.include_router(files_router)
app.include_router(photos_router)
app.include_router(profile_router)
app.include_router(consent_router)
app.include_router(onboarding_router)
app.include_router(proactive_router)
if settings.ENABLE_SMART_HOME == "1":
    app.include_router(smart_home_router)
    app.include_router(admin_smart_home_router)

# OpenTelemetry (optional)
setup_otel(app)

# Friendly root so Render URL doesn't show 404
@app.get("/")
def root():
    return {"ok": True, "service": "Executive AI Agent"}

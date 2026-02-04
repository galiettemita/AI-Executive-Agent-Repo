# app/main.py
from __future__ import annotations

import os
from dotenv import load_dotenv

# Load env FIRST (before importing modules that read env vars)
load_dotenv()

from fastapi import FastAPI
from fastapi.middleware.cors import CORSMiddleware

# Import DB AFTER env is loaded so DATABASE_URL is correct
from app.db.database import Base, engine  # noqa: E402
import app.db.models  # noqa: F401, E402

from app.api.routes.health import router as health_router  # noqa: E402
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


app = FastAPI(
    title="Shopping Assistant Backend",
    version="0.1.0",
)

# --- Optional CORS (safe defaults; tighten later) ---
# Set CORS_ORIGINS="https://yourdomain.com,https://www.yourdomain.com"
cors_origins_raw = os.getenv("CORS_ORIGINS", "")
origins = [o.strip() for o in cors_origins_raw.split(",") if o.strip()]
if origins:
    app.add_middleware(
        CORSMiddleware,
        allow_origins=origins,
        allow_credentials=True,
        allow_methods=["*"],
        allow_headers=["*"],
    )

# Routers
app.include_router(health_router, tags=["health"])
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

# Friendly root so Render URL doesn't show 404
@app.get("/")
def root():
    return {"ok": True, "service": "Shopping Assistant Backend"}

# Create tables at startup (MVP). Later replace with Alembic migrations.

# Global scheduler instance
_scheduler = None

@app.on_event("startup")
def on_startup():
    global _scheduler

    if os.getenv("ENABLE_CREATE_ALL") == "1":
        from app.db.database import Base, engine
        Base.metadata.create_all(bind=engine)

    # Start background scheduler for daily briefs
    if os.getenv("ENABLE_SCHEDULER", "1") == "1":
        from app.services.scheduler import start_scheduler
        _scheduler = start_scheduler()


@app.on_event("shutdown")
def on_shutdown():
    global _scheduler

    if _scheduler:
        from app.services.scheduler import stop_scheduler
        stop_scheduler(_scheduler)

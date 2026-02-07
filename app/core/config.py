# app/core/config.py
# Centralized configuration — all env vars live here.
# Services import `settings` instead of calling os.getenv() directly.

from __future__ import annotations

import logging
import sys

from pydantic_settings import BaseSettings, SettingsConfigDict

logger = logging.getLogger(__name__)


class Settings(BaseSettings):
    model_config = SettingsConfigDict(env_file=".env", extra="ignore")

    # ── Environment ─────────────────────────────────────────────
    ENV: str = "dev"  # "dev" | "staging" | "production"

    # ── Core ────────────────────────────────────────────────────
    DATABASE_URL: str = "sqlite:///./app.db"
    JWT_SECRET: str = "dev_only_change_me"
    APP_BASE_URL: str = "https://ai-shopping-assistant-backend-6bgf.onrender.com"

    # ── OpenAI ──────────────────────────────────────────────────
    OPENAI_API_KEY: str | None = None
    OPENAI_MODEL: str = "gpt-4o-mini"

    # ── Stripe ──────────────────────────────────────────────────
    STRIPE_SECRET_KEY: str | None = None
    STRIPE_WEBHOOK_SECRET: str | None = None
    STRIPE_PUBLISHABLE_KEY: str | None = None
    STRIPE_PRICE_ID_STARTER: str | None = None
    CHECKOUT_SUCCESS_URL: str = ""
    CHECKOUT_CANCEL_URL: str = ""

    # ── Amadeus (Travel) ────────────────────────────────────────
    AMADEUS_API_KEY: str | None = None
    AMADEUS_API_SECRET: str | None = None

    # ── Google OAuth ────────────────────────────────────────────
    GOOGLE_CLIENT_ID: str | None = None
    GOOGLE_CLIENT_SECRET: str | None = None
    GOOGLE_REDIRECT_URI: str = ""
    STATE_SIGNING_SECRET: str | None = None
    TOKEN_ENCRYPTION_KEY: str | None = None

    # ── WhatsApp ────────────────────────────────────────────────
    WHATSAPP_TOKEN: str = ""
    WHATSAPP_VERIFY_TOKEN: str = ""
    WHATSAPP_PHONE_NUMBER_ID: str = ""

    # ── Twilio (Voice & SMS) ────────────────────────────────────
    TWILIO_ACCOUNT_SID: str | None = None
    TWILIO_AUTH_TOKEN: str | None = None
    TWILIO_PHONE_NUMBER: str | None = None

    # ── ElevenLabs (TTS) ────────────────────────────────────────
    ELEVENLABS_API_KEY: str | None = None
    ELEVENLABS_DEFAULT_VOICE: str = "21m00Tcm4TlvDq8ikWAM"  # Rachel
    ELEVENLABS_VOICE_ID: str = "21m00Tcm4TlvDq8ikWAM"
    ELEVENLABS_MODEL_ID: str = "eleven_multilingual_v2"

    # ── Deepgram (STT) ─────────────────────────────────────────
    DEEPGRAM_API_KEY: str | None = None

    # ── Voice settings ──────────────────────────────────────────
    VOICE_CALL_MAX_DURATION_SECONDS: int = 1800  # 30 minutes

    # ── SerpAPI (Discovery) ─────────────────────────────────────
    SERPAPI_API_KEY: str | None = None
    SERPAPI_ENGINE: str = "google"
    SERPAPI_GL: str = "us"
    SERPAPI_HL: str = "en"

    # ── Security & Encryption ───────────────────────────────────
    PII_ENCRYPTION_KEY: str | None = None

    # ── Email (SMTP) ────────────────────────────────────────────
    SMTP_HOST: str = "smtp.gmail.com"
    SMTP_PORT: int = 587
    SMTP_USER: str = ""
    SMTP_PASSWORD: str = ""
    FROM_EMAIL: str = "noreply@yourassistant.com"
    FROM_NAME: str = "Executive AI Agent"

    # ── CORS ────────────────────────────────────────────────────
    CORS_ORIGINS: str = ""

    # ── Scheduler ───────────────────────────────────────────────
    ENABLE_SCHEDULER: str = "1"
    ENABLE_CREATE_ALL: str = "0"
    DAILY_BRIEF_SCHEDULE: str = "7 0"
    PRICE_MONITORING_INTERVAL_MINUTES: int = 60
    NOTIFICATION_DELIVERY_INTERVAL_MINUTES: int = 5

    # ── History ─────────────────────────────────────────────────
    HISTORY_TURNS: int = 6

    # ── Circuit breaker ─────────────────────────────────────────
    CIRCUIT_BREAKER_FAILURE_THRESHOLD: int = 5
    CIRCUIT_BREAKER_RECOVERY_TIMEOUT: int = 30
    CIRCUIT_BREAKER_SUCCESS_THRESHOLD: int = 3

    # ── Rate limiting ───────────────────────────────────────────
    REDIS_URL: str | None = None
    RATE_LIMIT_USER: str = "10/minute"
    RATE_LIMIT_IP: str = "100/minute"

    # ── Foundation (future) ─────────────────────────────────────
    MONGODB_URI: str | None = None
    CELERY_BROKER_URL: str | None = None
    SENTRY_DSN: str | None = None


settings = Settings()


# ── Production guards (run at import time) ──────────────────────
def _validate_settings():
    """Warn or fail on dangerous configuration in non-dev environments."""
    if settings.ENV in ("production", "staging"):
        if settings.JWT_SECRET == "dev_only_change_me":
            logger.critical(
                "JWT_SECRET is still the default value in %s environment. "
                "Set a secure random JWT_SECRET before deploying.",
                settings.ENV,
            )
            sys.exit(1)

        if settings.DATABASE_URL.startswith("sqlite"):
            logger.critical(
                "DATABASE_URL points to SQLite in %s environment. "
                "Use PostgreSQL for production.",
                settings.ENV,
            )
            sys.exit(1)

    # Warnings for missing optional-but-important keys
    if not settings.OPENAI_API_KEY:
        logger.warning("OPENAI_API_KEY not set — AI features will not work")
    if not settings.PII_ENCRYPTION_KEY:
        logger.warning("PII_ENCRYPTION_KEY not set — PII encryption will fall back to JWT_SECRET")


_validate_settings()

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
    APP_VERSION: str = ""

    # ── OpenAI ──────────────────────────────────────────────────
    OPENAI_API_KEY: str | None = None
    OPENAI_MODEL: str = "gpt-4o-mini"
    OPENAI_EMBEDDING_MODEL: str = "text-embedding-3-small"

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

    # ── Microsoft OAuth ────────────────────────────────────────
    MS_CLIENT_ID: str | None = None
    MS_CLIENT_SECRET: str | None = None
    MS_REDIRECT_URI: str = ""
    MS_TENANT_ID: str = "common"

    # ── WhatsApp ────────────────────────────────────────────────
    WHATSAPP_TOKEN: str = ""
    WHATSAPP_VERIFY_TOKEN: str = ""
    WHATSAPP_PHONE_NUMBER_ID: str = ""
    WHATSAPP_APP_SECRET: str = ""
    WHATSAPP_PUBLIC_NUMBER: str = ""  # E.164 without "+" for wa.me links

    # ── Fitbit OAuth ────────────────────────────────────────────
    FITBIT_CLIENT_ID: str | None = None
    FITBIT_CLIENT_SECRET: str | None = None
    FITBIT_REDIRECT_URI: str = ""

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
    VOICE_CALL_AUTO_EXECUTE_ON_APPROVAL: str = "1"
    ENABLE_VOICE_CALLS: str = "0"
    VOICE_RECORDING_RETENTION_DAYS: int = 30

    # ── SerpAPI (Discovery) ─────────────────────────────────────
    SERPAPI_API_KEY: str | None = None
    SERPAPI_ENGINE: str = "google"
    SERPAPI_GL: str = "us"
    SERPAPI_HL: str = "en"

    # ── Security & Encryption ───────────────────────────────────
    PII_ENCRYPTION_KEY: str | None = None
    PII_ENCRYPTION_KEYS: str | None = None
    ENFORCE_WEBHOOK_SIGNATURES: str = "1"
    AUDIT_LOG_ENABLED: str = "1"
    SECURITY_HEADERS_ENABLED: str = "1"
    SECURITY_HSTS_ENABLED: str = "1"
    SECURITY_HSTS_MAX_AGE: int = 31536000
    SECURITY_CSP_REPORT_ONLY: str = "0"
    SECURITY_CSP: str = (
        "default-src 'self'; "
        "img-src 'self' data: https:; "
        "style-src 'self' 'unsafe-inline' https://fonts.googleapis.com; "
        "font-src 'self' https://fonts.gstatic.com; "
        "script-src 'self' 'unsafe-inline'; "
        "connect-src 'self' https:; "
        "frame-ancestors 'none'; "
        "base-uri 'self'; "
        "form-action 'self'"
    )

    # ── Email (SMTP) ────────────────────────────────────────────
    SMTP_HOST: str = "smtp.gmail.com"
    SMTP_PORT: int = 587
    SMTP_USER: str = ""
    SMTP_PASSWORD: str = ""
    FROM_EMAIL: str = "noreply@yourassistant.com"
    FROM_NAME: str = "Executive AI Agent"

    # ── CORS ────────────────────────────────────────────────────
    CORS_ORIGINS: str = ""

    # ── Public Site ─────────────────────────────────────────────
    PUBLIC_SITE_NAME: str = "Executive AI Agent"
    PUBLIC_SITE_TAGLINE: str = "Your WhatsApp-first executive assistant"
    PUBLIC_SITE_SUPPORT_EMAIL: str = "support@yourassistant.com"

    # ── Weather (Wardrobe) ─────────────────────────────────────
    WEATHER_PROVIDER: str = "open_meteo"  # open_meteo | openweather | weatherapi
    WEATHER_API_KEY: str | None = None
    WEATHER_DEFAULT_LOCATION: str = ""

    # ── Scheduler ───────────────────────────────────────────────
    ENABLE_SCHEDULER: str = "1"
    ENABLE_CREATE_ALL: str = "0"
    DAILY_BRIEF_SCHEDULE: str = "7 0"
    PRICE_MONITORING_INTERVAL_MINUTES: int = 60
    NOTIFICATION_DELIVERY_INTERVAL_MINUTES: int = 5
    ENERGY_MONITOR_INTERVAL_MINUTES: int = 15
    PROACTIVE_RULE_POLL_MINUTES: int = 5
    EMAIL_MONITOR_INTERVAL_MINUTES: int = 10
    EMAIL_MONITOR_TEST_MODE: str = "0"

    # ── Onboarding / Phone verification ─────────────────────────
    REQUIRE_PHONE_VERIFICATION: str = "0"
    PHONE_VERIFICATION_CODE_LENGTH: int = 6
    PHONE_VERIFICATION_CODE_TTL_MINUTES: int = 10
    PHONE_VERIFICATION_MAX_ATTEMPTS: int = 5
    PHONE_VERIFICATION_RESEND_COOLDOWN_SECONDS: int = 60
    PHONE_VERIFICATION_ALLOW_DEV_CODE_ECHO: str = "1"

    # ── History ─────────────────────────────────────────────────
    HISTORY_TURNS: int = 6

    # ── Wardrobe ────────────────────────────────────────────────
    WARDROBE_LLM_ENABLED: str = "1"
    WARDROBE_ROTATION_DAYS: int = 30
    WARDROBE_ROTATION_COOLDOWN_DAYS: int = 7
    WARDROBE_ROTATION_SCHEDULE: str = "8 0"
    WARDROBE_ROTATION_MAX_ITEMS: int = 5
    WARDROBE_WEAR_LOOKBACK_DAYS: int = 90
    WARDROBE_SHOPPING_MAX_RESULTS: int = 6

    # ── Gifts ───────────────────────────────────────────────────
    GIFT_LLM_ENABLED: str = "1"
    GIFT_REMINDER_SCHEDULE: str = "9 0"
    GIFT_REMINDER_DEFAULT_DAYS: int = 14
    GIFT_SHOPPING_MAX_RESULTS: int = 6

    # ── Relationships ───────────────────────────────────────────
    RELATIONSHIP_DEFAULT_CADENCE_DAYS: int = 30
    RELATIONSHIP_REMINDER_SCHEDULE: str = "10 0"
    RELATIONSHIP_REMINDER_MAX_PER_USER: int = 10

    # ── Fitness & Nutrition ────────────────────────────────────
    FITNESS_DEFAULT_CALORIES: int = 2000
    FITNESS_PROTEIN_RATIO: float = 0.3
    FITNESS_CARBS_RATIO: float = 0.4
    FITNESS_FAT_RATIO: float = 0.3

    # ── Circuit breaker ─────────────────────────────────────────
    CIRCUIT_BREAKER_FAILURE_THRESHOLD: int = 5
    CIRCUIT_BREAKER_RECOVERY_TIMEOUT: int = 30
    CIRCUIT_BREAKER_SUCCESS_THRESHOLD: int = 3

    # ── Rate limiting ───────────────────────────────────────────
    REDIS_URL: str | None = None
    RATE_LIMIT_USER: str = "10/minute"
    RATE_LIMIT_IP: str = "100/minute"
    REDIS_SESSION_TTL_SECONDS: int = 60 * 60 * 24  # 24h
    REDIS_PREFS_TTL_SECONDS: int = 60 * 60 * 6  # 6h
    REDIS_ENTITLEMENTS_TTL_SECONDS: int = 60 * 5  # 5m
    BETA_MODE: str = "0"
    BETA_ALLOWED_USER_IDS: str = ""

    # ── Foundation (future) ─────────────────────────────────────
    MONGODB_URI: str | None = None
    MONGODB_DB: str = "executive_ai_agent"
    CELERY_BROKER_URL: str | None = None
    CELERY_RESULT_BACKEND: str | None = None
    CELERY_TASK_ALWAYS_EAGER: bool = False
    SENTRY_DSN: str | None = None

    # ── Observability ───────────────────────────────────────────
    PROMETHEUS_ENABLED: bool = False
    METRICS_TOKEN: str | None = None
    SENTRY_TRACES_SAMPLE_RATE: float = 0.05
    SENTRY_PROFILES_SAMPLE_RATE: float = 0.0

    OTEL_ENABLED: bool = False
    OTEL_SERVICE_NAME: str = "executive-ai-agent"
    OTEL_EXPORTER_OTLP_ENDPOINT: str | None = None
    OTEL_EXPORTER_OTLP_HEADERS: str | None = None

    # ── Storage ────────────────────────────────────────────────
    STORAGE_BACKEND: str = "local"  # local | s3
    LOCAL_STORAGE_PATH: str = "./storage"
    S3_BUCKET: str | None = None
    S3_REGION: str | None = None
    S3_ACCESS_KEY_ID: str | None = None
    S3_SECRET_ACCESS_KEY: str | None = None
    S3_ENDPOINT_URL: str | None = None

    # ── Vector DB ──────────────────────────────────────────────
    VECTOR_DB_BACKEND: str | None = None  # pinecone | weaviate | pgvector
    PINECONE_API_KEY: str | None = None
    PINECONE_ENVIRONMENT: str | None = None
    PINECONE_INDEX: str | None = None
    WEAVIATE_URL: str | None = None
    WEAVIATE_API_KEY: str | None = None
    PGVECTOR_DSN: str | None = None

    # ── Semantic Search / Vision ───────────────────────────────
    FILE_EMBEDDINGS_ENABLED: str = "0"
    FILE_EMBEDDINGS_MAX_CHARS: int = 6000
    PHOTO_EMBEDDINGS_ENABLED: str = "0"
    PHOTO_TAGGING_ENABLED: str = "0"
    PHOTO_TAGGING_MAX_BYTES: int = 4_000_000
    OPENAI_VISION_MODEL: str = "gpt-4o-mini"

    # ── Alerting ───────────────────────────────────────────────
    ALERTING_PROVIDER: str | None = None  # sentry | slack | pagerduty
    SLACK_ALERT_WEBHOOK_URL: str | None = None
    PAGERDUTY_ROUTING_KEY: str | None = None

    # ── Smart Home ──────────────────────────────────────────────
    SMART_HOME_DEFAULT_PROVIDER: str = "home_assistant"
    ENABLE_SMART_HOME: str = "0"
    ENABLE_MESSAGING: str = "0"


settings = Settings()


# ── Production guards (run at import time) ──────────────────────
def _validate_settings():
    """Warn or fail on dangerous configuration in non-dev environments."""
    if settings.ENV not in ("dev", "staging", "production"):
        logger.critical("ENV must be one of dev, staging, production. Got: %s", settings.ENV)
        sys.exit(1)

    if settings.ENV in ("production", "staging"):
        critical_missing: list[str] = []

        if settings.JWT_SECRET == "dev_only_change_me" or not settings.JWT_SECRET:
            critical_missing.append("JWT_SECRET")

        if settings.DATABASE_URL.startswith("sqlite"):
            critical_missing.append("DATABASE_URL (must be PostgreSQL)")

        if not settings.OPENAI_API_KEY:
            critical_missing.append("OPENAI_API_KEY")

        if not settings.PII_ENCRYPTION_KEY:
            critical_missing.append("PII_ENCRYPTION_KEY")

        if critical_missing:
            logger.critical(
                "Missing or invalid critical settings in %s: %s",
                settings.ENV,
                ", ".join(critical_missing),
            )
            sys.exit(1)

    # Warnings for missing optional-but-important keys
    if not settings.OPENAI_API_KEY:
        logger.warning("OPENAI_API_KEY not set — AI features will not work")
    if not settings.PII_ENCRYPTION_KEY:
        logger.warning(
            "PII_ENCRYPTION_KEY not set — PII encryption will fall back to JWT_SECRET"
        )


_validate_settings()

# app/core/config.py
# Centralized configuration — all env vars live here.
# Services import `settings` instead of calling os.getenv() directly.

from __future__ import annotations

import logging
import sys

from pydantic import AliasChoices, Field
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
    CLERK_SECRET_KEY: str | None = None

    # ── OpenAI ──────────────────────────────────────────────────
    OPENAI_API_KEY: str | None = None
    OPENAI_MODEL: str = "gpt-4o-mini"
    OPENAI_EMBEDDING_MODEL: str = "text-embedding-3-small"
    OPENAI_ORG_ID: str | None = None

    # ── Multi-Provider LLM Router (Blueprint v5 Section 9) ─────
    ANTHROPIC_API_KEY: str | None = None
    GOOGLE_AI_API_KEY: str | None = None
    LOCAL_LLM_ENDPOINT: str | None = None
    LLM_ROUTER_FAILOVER_TIMEOUT_S: int = 30
    LLM_ROUTER_HEALTH_CHECK_INTERVAL: int = 30

    # ── Stripe ──────────────────────────────────────────────────
    STRIPE_SECRET_KEY: str | None = None
    STRIPE_WEBHOOK_SECRET: str | None = None
    STRIPE_PUBLISHABLE_KEY: str | None = None
    STRIPE_PRICE_ID_STARTER: str | None = None
    STRIPE_PRICE_ID_PERSONAL: str | None = None
    STRIPE_PRICE_ID_PROFESSIONAL: str | None = None
    CHECKOUT_SUCCESS_URL: str = ""
    CHECKOUT_CANCEL_URL: str = ""
    BILLING_PORTAL_RETURN_URL: str = ""

    # ── Billing (Operational Blueprint Component 1) ─────────────
    BILLING_TRIAL_DAYS: int = 14
    BILLING_TRIAL_DAILY_MESSAGE_LIMIT: int = 20
    BILLING_TRIAL_MAX_MCP_SERVERS: int = 3
    BILLING_BURST_LIMIT_PER_MINUTE: int = 10
    BILLING_BURST_WINDOW_SECONDS: int = 60
    BILLING_PAST_DUE_GRACE_DAYS: int = 7
    # 0 means unlimited.
    BILLING_MCP_MONTHLY_LIMIT_CENTS_TRIAL: int = 500
    BILLING_MCP_MONTHLY_LIMIT_CENTS_PERSONAL: int = 5000
    BILLING_MCP_MONTHLY_LIMIT_CENTS_PROFESSIONAL: int = 25000
    BILLING_MCP_MONTHLY_LIMIT_CENTS_ENTERPRISE: int = 0

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
    MS_CLIENT_ID: str | None = Field(
        default=None,
        validation_alias=AliasChoices("MS_CLIENT_ID", "MICROSOFT_CLIENT_ID"),
    )
    MS_CLIENT_SECRET: str | None = Field(
        default=None,
        validation_alias=AliasChoices("MS_CLIENT_SECRET", "MICROSOFT_CLIENT_SECRET"),
    )
    MS_REDIRECT_URI: str = ""
    MS_TENANT_ID: str = "common"

    # ── WhatsApp ────────────────────────────────────────────────
    # Accept legacy `WA_*` env var names used by current infra/secrets.
    WHATSAPP_TOKEN: str = Field(
        default="",
        validation_alias=AliasChoices("WHATSAPP_TOKEN", "WA_ACCESS_TOKEN"),
    )
    WHATSAPP_VERIFY_TOKEN: str = Field(
        default="",
        validation_alias=AliasChoices("WHATSAPP_VERIFY_TOKEN", "WA_VERIFY_TOKEN"),
    )
    WHATSAPP_PHONE_NUMBER_ID: str = Field(
        default="",
        validation_alias=AliasChoices("WHATSAPP_PHONE_NUMBER_ID", "WA_PHONE_NUMBER_ID"),
    )
    WHATSAPP_APP_SECRET: str = Field(
        default="",
        validation_alias=AliasChoices("WHATSAPP_APP_SECRET", "WA_APP_SECRET"),
    )
    WHATSAPP_BUSINESS_ACCOUNT_ID: str = Field(
        default="",
        validation_alias=AliasChoices("WHATSAPP_BUSINESS_ACCOUNT_ID", "WA_BUSINESS_ACCOUNT_ID"),
    )
    WHATSAPP_PUBLIC_NUMBER: str = ""  # E.164 without "+" for wa.me links

    # ── Slack (Phase 3) ────────────────────────────────────────
    SLACK_CLIENT_ID: str | None = None
    SLACK_CLIENT_SECRET: str | None = None
    SLACK_REDIRECT_URI: str | None = None
    SLACK_BOT_TOKEN: str | None = None
    SLACK_SIGNING_SECRET: str | None = None

    # ── Apple Messages for Business / iMessage (Phase 3) ───────
    IMESSAGE_CERT_VALIDATION_REQUIRED: str = "0"
    IMESSAGE_CERT_SHA256_ALLOWLIST: str | None = None
    IMESSAGE_TRUSTED_ROOT_PEM: str | None = None

    # ── Plaid (Phase 3+) ────────────────────────────────────────
    PLAID_CLIENT_ID: str | None = None
    PLAID_SECRET_STAGING: str | None = None
    PLAID_SECRET_PROD: str | None = None
    PLAID_ENV_STAGING: str = "sandbox"
    PLAID_ENV_PROD: str = "production"
    PLAID_REDIRECT_URI_STAGING: str | None = None
    PLAID_REDIRECT_URI_PROD: str | None = None
    PLAID_WEBHOOK_SECRET: str | None = None

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

    # ── Tavily (Discovery) ──────────────────────────────────────
    TAVILY_API_KEY: str | None = None
    TAVILY_SEARCH_DEPTH: str = "basic"  # basic | advanced

    # ── Wave 1 MCP Provider Keys ───────────────────────────────
    BRAVE_API_KEY: str | None = None
    NOTION_API_KEY: str | None = None
    TODOIST_API_TOKEN: str | None = None
    GITHUB_APP_ID: str | None = None
    GITHUB_APP_PRIVATE_KEY: str | None = None
    GITHUB_INSTALLATION_ID: str | None = None

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

    # ── Email ───────────────────────────────────────────────────
    EMAIL_PROVIDER: str = "ses"  # smtp | ses
    SMTP_HOST: str = "smtp.gmail.com"
    SMTP_PORT: int = 587
    SMTP_USER: str = ""
    SMTP_PASSWORD: str = ""
    SES_REGION: str | None = None
    SES_CONFIGURATION_SET: str | None = None
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
    DATA_RETENTION_SCHEDULE: str = "3 30"

    # ── Data Retention ──────────────────────────────────────────
    RETENTION_NOTIFICATION_QUEUE_DAYS: int = 30
    RETENTION_OUTBOUND_MESSAGES_DAYS: int = 30
    RETENTION_CHAT_MESSAGES_DAYS: int = 180
    RETENTION_EMAIL_ALERTS_DAYS: int = 180
    RETENTION_EMAIL_DRAFTS_DAYS: int = 90
    RETENTION_WEBHOOK_DELIVERIES_DAYS: int = 30
    RETENTION_USAGE_EVENTS_DAYS: int = 730
    DATA_RETENTION_DAYS_CANCELED: int = 90
    RETENTION_AUDIT_LOGS_DAYS: int = 365
    RETENTION_WATCH_OFFERS_DAYS: int = 90
    RETENTION_SMART_HOME_READINGS_DAYS: int = 90

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
    CELERY_BROKER_URL: str | None = None
    CELERY_RESULT_BACKEND: str | None = None
    CELERY_TASK_ALWAYS_EAGER: bool = False
    SENTRY_DSN: str | None = None

    # ── Internal Plane URLs (Gateway/Workers → Brain/Hands) ─────
    BRAIN_INTERNAL_BASE_URL: str | None = None
    HANDS_INTERNAL_BASE_URL: str | None = None

    # ── Observability ───────────────────────────────────────────
    PROMETHEUS_ENABLED: bool = False
    METRICS_TOKEN: str | None = None
    SENTRY_TRACES_SAMPLE_RATE: float = 0.05
    SENTRY_PROFILES_SAMPLE_RATE: float = 0.0

    OTEL_ENABLED: bool = False
    OTEL_SERVICE_NAME: str = "executive-ai-agent"
    OTEL_EXPORTER_OTLP_ENDPOINT: str | None = None
    OTEL_EXPORTER_OTLP_HEADERS: str | None = None
    OTEL_EXPORTER_OTLP_TRACES_ENDPOINT: str | None = None
    OTEL_EXPORTER_OTLP_TRACES_HEADERS: str | None = None
    OTEL_EXPORTER_OTLP_METRICS_ENDPOINT: str | None = None
    OTEL_EXPORTER_OTLP_METRICS_HEADERS: str | None = None
    OTEL_METRICS_ENABLED: bool = True

    # ── Semantic Cache ──────────────────────────────────────────
    SEMANTIC_CACHE_ENABLED: bool = True
    SEMANTIC_CACHE_TTL_SECONDS: int = 60 * 60 * 6
    SEMANTIC_CACHE_MIN_SIMILARITY: float = 0.92
    SEMANTIC_CACHE_MIN_QUERY_CHARS: int = 16

    # ── Product Analytics ───────────────────────────────────────
    POSTHOG_API_KEY: str | None = None
    POSTHOG_HOST: str = "https://app.posthog.com"

    # ── Feature Flags (Blueprint Appendix A) ────────────────────
    FEATURE_TIER_3_ENABLED: bool = True
    FEATURE_PROACTIVE_ENABLED: bool = True
    FEATURE_IMESSAGE_ENABLED: bool = False
    FEATURE_BEHAVIORAL_INTELLIGENCE: bool = True
    FEATURE_PROFILING_ENABLED: bool = True
    FEATURE_CONSOLIDATION_ENABLED: bool = True
    FEATURE_SELF_REVIEW_ENABLED: bool = True
    FEATURE_MULTI_PROVIDER_LLM: bool = True
    FEATURE_VOICE_INPUT: bool = True
    FEATURE_VOICE_OUTPUT: bool = False
    FEATURE_IMAGE_PROCESSING: bool = True
    FEATURE_DOCUMENT_PROCESSING: bool = True
    FEATURE_TEAM_DELEGATION: bool = False
    FEATURE_RESEARCH_ENGINE: bool = False
    FEATURE_WORKFLOWS: bool = False
    FEATURE_MCP_CLIENT: bool = True
    FEATURE_EMOTION_DETECTION: bool = True
    FEATURE_PRIVILEGE_ISOLATION: bool = False
    FEATURE_DOCUMENT_GENERATION: bool = False
    FEATURE_CROSS_CHANNEL_CONTINUITY: bool = False
    FEATURE_AB_TESTING: bool = False

    # ── MCP Sandbox Controls ───────────────────────────────────
    MCP_NETWORK_ALLOWLIST: str = ""
    MCP_STDIO_ALLOWED_COMMANDS: str = ""
    MCP_HOST_TOKEN: str | None = None
    MCP_WAVE1_TRANSPORT_MODE: str = "mock"  # mock | streamable_http | stdio
    MCP_GOOGLE_CALENDAR_URL: str | None = None
    MCP_GOOGLE_DRIVE_URL: str | None = None
    MCP_GMAIL_URL: str | None = None
    MCP_NOTION_URL: str | None = None
    MCP_TODOIST_URL: str | None = None
    MCP_BRAVE_SEARCH_URL: str | None = None
    MCP_GITHUB_URL: str | None = None
    MCP_APPLE_REMINDERS_URL: str | None = None

    # ── Temporal Orchestration (Tier 3) ────────────────────────
    TEMPORAL_ENABLED: bool = False
    TEMPORAL_HOST: str | None = None
    TEMPORAL_NAMESPACE: str = "default"
    TEMPORAL_TASK_QUEUE_TIER3: str = "executive-os-tier3"
    TEMPORAL_TIER3_WORKFLOW_NAME: str = "Tier3PlannerWorkflow"
    TEMPORAL_WORKFLOW_TIMEOUT_S: int = 120

    # ── Storage ────────────────────────────────────────────────
    STORAGE_BACKEND: str = "local"  # local | s3
    LOCAL_STORAGE_PATH: str = "./storage"
    S3_BUCKET: str | None = None
    S3_KNOWLEDGE_BUCKET: str | None = None
    S3_VOICE_BUCKET: str | None = None
    S3_DOCUMENTS_BUCKET: str | None = None
    S3_REGION: str | None = None
    AWS_REGION: str | None = None
    S3_ACCESS_KEY_ID: str | None = None
    S3_SECRET_ACCESS_KEY: str | None = None
    S3_ENDPOINT_URL: str | None = None

    # ── Vector DB ──────────────────────────────────────────────
    VECTOR_DB_BACKEND: str | None = None  # pgvector
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

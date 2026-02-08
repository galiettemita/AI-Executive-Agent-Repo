# Render Environment Variables

This document lists all environment variables that must be configured in your Render dashboard.

## Required Environment Variables

### Core Application
```
ENV=production
DATABASE_URL=<your-render-postgres-url>
JWT_SECRET=<generate-a-secure-random-string>
PII_ENCRYPTION_KEY=<generate-with-fernet-key-generator>
APP_BASE_URL=https://your-render-domain.onrender.com
```

### WhatsApp Integration
```
WHATSAPP_VERIFY_TOKEN=<your-whatsapp-verify-token>
WHATSAPP_TOKEN=<your-whatsapp-access-token>
WHATSAPP_PHONE_NUMBER_ID=<your-whatsapp-phone-number-id>
```

### OpenAI
```
OPENAI_API_KEY=<your-openai-api-key>
OPENAI_MODEL=gpt-4o-mini
OPENAI_EMBEDDING_MODEL=text-embedding-3-small
OPENAI_VISION_MODEL=gpt-4o-mini
```

### Semantic Search / Vision
```
FILE_EMBEDDINGS_ENABLED=1
FILE_EMBEDDINGS_MAX_CHARS=6000
PHOTO_EMBEDDINGS_ENABLED=1
PHOTO_TAGGING_ENABLED=1
PHOTO_TAGGING_MAX_BYTES=4000000
```

### Voice Calling (Stage 18)
```
APP_BASE_URL=https://your-render-domain.onrender.com
TWILIO_ACCOUNT_SID=<your-twilio-account-sid>
TWILIO_AUTH_TOKEN=<your-twilio-auth-token>
TWILIO_PHONE_NUMBER=<your-twilio-number>
DEEPGRAM_API_KEY=<your-deepgram-api-key>
ELEVENLABS_API_KEY=<your-elevenlabs-api-key>
ELEVENLABS_VOICE_ID=<default-voice-id>
ELEVENLABS_MODEL_ID=eleven_multilingual_v2
VOICE_CALL_AUTO_EXECUTE_ON_APPROVAL=1
ENABLE_VOICE_CALLS=0
VOICE_RECORDING_RETENTION_DAYS=30
```

### Google OAuth (Stage 5)
**IMPORTANT**: Update redirect URI to point to your Render domain
```
GOOGLE_CLIENT_ID=<your-google-client-id>
GOOGLE_CLIENT_SECRET=<your-google-client-secret>
GOOGLE_REDIRECT_URI=https://ai-shopping-assistant-backend-6bgf.onrender.com/admin/google/callback
TOKEN_ENCRYPTION_KEY=<generate-with-fernet-key-generator>
```

**Note**: You must also update this redirect URI in your Google Cloud Console:
1. Go to https://console.cloud.google.com/apis/credentials
2. Select your OAuth 2.0 Client ID
3. Add `https://ai-shopping-assistant-backend-6bgf.onrender.com/admin/google/callback` to "Authorized redirect URIs"
4. Save changes

### Microsoft OAuth (Outlook + Calendar)
```
MS_CLIENT_ID=<your-azure-app-client-id>
MS_CLIENT_SECRET=<your-azure-app-client-secret>
MS_REDIRECT_URI=https://ai-shopping-assistant-backend-6bgf.onrender.com/admin/microsoft/callback
MS_TENANT_ID=common   # or your tenant ID for single-tenant apps
```

**Note**: You must also add the redirect URI in Azure:
1. Go to Azure Portal → App registrations
2. Select your app → Authentication
3. Add the redirect URI above under “Web”
4. Save changes

### Apple/iCloud CalDAV
CalDAV credentials are stored per-user (not global env vars). Use the CalDAV connect endpoint:
- POST `/admin/caldav/connect`
- Provide `server_url`, `username`, and an app-specific password

### Security & Encryption
```
STATE_SIGNING_SECRET=<generate-a-secure-random-string>
TOKEN_ENCRYPTION_KEY=<generate-with-fernet-key-generator>
PII_ENCRYPTION_KEYS=<comma-separated-old-and-new-keys-for-rotation>
ENFORCE_WEBHOOK_SIGNATURES=1
AUDIT_LOG_ENABLED=1
```

To generate TOKEN_ENCRYPTION_KEY:
```python
from cryptography.fernet import Fernet
print(Fernet.generate_key().decode())
```
You can use the same method for `PII_ENCRYPTION_KEY`.

### Stripe (Stage 3)
```
STRIPE_SECRET_KEY=<your-stripe-secret-key>
STRIPE_WEBHOOK_SECRET=<your-stripe-webhook-secret>
STRIPE_PRICE_ID_STARTER=<your-stripe-price-id>
CHECKOUT_SUCCESS_URL=https://ai-shopping-assistant-backend-6bgf.onrender.com/billing/success
CHECKOUT_CANCEL_URL=https://ai-shopping-assistant-backend-6bgf.onrender.com/billing/cancel
```

### SerpAPI (Product Discovery)
```
SERPAPI_API_KEY=<your-serpapi-key>
SERPAPI_ENGINE=google
SERPAPI_GL=us
SERPAPI_HL=en
```

### CORS (Stage 4)
```
CORS_ORIGINS=https://yourdomain.com,https://www.yourdomain.com
```

### Redis
```
REDIS_URL=<your-redis-url>
```
Required in `staging` and `production` for distributed rate limiting and caching.

### Celery
```
CELERY_BROKER_URL=<broker-url>        # optional if using REDIS_URL
CELERY_RESULT_BACKEND=<backend-url>   # optional
```

### MongoDB
```
MONGODB_URI=<mongodb-uri>
MONGODB_DB=executive_ai_agent
```

### Observability
```
PROMETHEUS_ENABLED=1
METRICS_TOKEN=<random-token-for-metrics-endpoint>
SENTRY_DSN=<sentry-dsn>
```

### OpenTelemetry (optional)
```
OTEL_ENABLED=1
OTEL_SERVICE_NAME=executive-ai-agent
OTEL_EXPORTER_OTLP_ENDPOINT=<otel-collector-endpoint>
OTEL_EXPORTER_OTLP_HEADERS=Authorization=Bearer <token>
```

### Object Storage
```
STORAGE_BACKEND=s3
S3_BUCKET=<bucket-name>
S3_REGION=<region>
S3_ACCESS_KEY_ID=<access-key>
S3_SECRET_ACCESS_KEY=<secret-key>
S3_ENDPOINT_URL=<optional-custom-endpoint>
```

### Vector DB
```
VECTOR_DB_BACKEND=pinecone|weaviate|pgvector
PINECONE_API_KEY=<key>
PINECONE_ENVIRONMENT=<region>
PINECONE_INDEX=<index>
WEAVIATE_URL=<url>
WEAVIATE_API_KEY=<key>
PGVECTOR_DSN=<postgres-connection-string>
```

### Alerting
```
ALERTING_PROVIDER=sentry|slack|pagerduty
SLACK_ALERT_WEBHOOK_URL=<webhook-url>
PAGERDUTY_ROUTING_KEY=<routing-key>
```

### Scheduler (Stage 5)
```
ENABLE_SCHEDULER=1
DAILY_BRIEF_SCHEDULE=7 0
ENERGY_MONITOR_INTERVAL_MINUTES=15
PROACTIVE_RULE_POLL_MINUTES=5
EMAIL_MONITOR_INTERVAL_MINUTES=10
EMAIL_MONITOR_TEST_MODE=0
WARDROBE_ROTATION_SCHEDULE=8 0
GIFT_REMINDER_SCHEDULE=9 0
```
Format: "hour minute" in UTC (e.g., "7 0" = 7:00 AM UTC)

### Weather + Wardrobe (Optional)
```
WEATHER_PROVIDER=open_meteo
WEATHER_DEFAULT_LOCATION=New York, NY
WARDROBE_LLM_ENABLED=1
WARDROBE_ROTATION_DAYS=30
WARDROBE_ROTATION_COOLDOWN_DAYS=7
WARDROBE_ROTATION_MAX_ITEMS=5
WARDROBE_WEAR_LOOKBACK_DAYS=90
WARDROBE_SHOPPING_MAX_RESULTS=6
```

### Gifts (Optional)
```
GIFT_LLM_ENABLED=1
GIFT_REMINDER_DEFAULT_DAYS=14
GIFT_SHOPPING_MAX_RESULTS=6
```

### Phone Verification / Onboarding
```
REQUIRE_PHONE_VERIFICATION=0
PHONE_VERIFICATION_CODE_LENGTH=6
PHONE_VERIFICATION_CODE_TTL_MINUTES=10
PHONE_VERIFICATION_MAX_ATTEMPTS=5
PHONE_VERIFICATION_RESEND_COOLDOWN_SECONDS=60
PHONE_VERIFICATION_ALLOW_DEV_CODE_ECHO=0
```

### Smart Home
```
SMART_HOME_DEFAULT_PROVIDER=home_assistant
ENABLE_SMART_HOME=0
```
Smart home provider credentials are stored per-user via `/admin/smart_home/connect` (Home Assistant).

### Messaging
```
ENABLE_MESSAGING=0
```

### Beta Gating (Optional)
```
BETA_MODE=0
BETA_ALLOWED_USER_IDS=user1,user2
```

## Optional Environment Variables

```
ENABLE_CREATE_ALL=0
WHATSAPP_APP_SECRET=<meta-app-secret>     # Enables webhook signature verification
APP_VERSION=<git-sha-or-release-tag>      # Optional health/version reporting
```
Set to `1` only if you want to create tables on startup (not recommended for production - use Alembic migrations instead)

---

## After Setting Environment Variables

1. Deploy your app to Render
2. Run database migrations:
   ```bash
   alembic upgrade head
   ```

3. Verify the service is running:
   ```bash
   curl https://ai-shopping-assistant-backend-6bgf.onrender.com/
   ```

4. Test Google OAuth flow:
   - GET `/admin/google/connect?user_id=test_user`
   - Follow the authorization flow
   - Should redirect to `/admin/google/callback`

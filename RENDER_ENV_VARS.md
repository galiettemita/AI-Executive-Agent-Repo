# Render Environment Variables

This document lists all environment variables that must be configured in your Render dashboard.

## Required Environment Variables

### Core Application
```
DATABASE_URL=<your-render-postgres-url>
JWT_SECRET=<generate-a-secure-random-string>
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
```

### Google OAuth (Stage 5)
**IMPORTANT**: Update redirect URI to point to your Render domain
```
GOOGLE_CLIENT_ID=<your-google-client-id>
GOOGLE_CLIENT_SECRET=<your-google-client-secret>
GOOGLE_REDIRECT_URI=https://ai-shopping-assistant-backend-6bgf.onrender.com/admin/google/callback
```

**Note**: You must also update this redirect URI in your Google Cloud Console:
1. Go to https://console.cloud.google.com/apis/credentials
2. Select your OAuth 2.0 Client ID
3. Add `https://ai-shopping-assistant-backend-6bgf.onrender.com/admin/google/callback` to "Authorized redirect URIs"
4. Save changes

### Security & Encryption
```
STATE_SIGNING_SECRET=<generate-a-secure-random-string>
TOKEN_ENCRYPTION_KEY=<generate-with-fernet-key-generator>
```

To generate TOKEN_ENCRYPTION_KEY:
```python
from cryptography.fernet import Fernet
print(Fernet.generate_key().decode())
```

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

### Scheduler (Stage 5)
```
ENABLE_SCHEDULER=1
DAILY_BRIEF_SCHEDULE=7 0
```
Format: "hour minute" in UTC (e.g., "7 0" = 7:00 AM UTC)

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

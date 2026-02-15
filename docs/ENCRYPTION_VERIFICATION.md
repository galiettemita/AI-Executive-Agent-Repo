# Encryption Verification Checklist

Use this checklist to confirm encryption at rest and in transit for all data stores.

## In Transit (TLS)
1. Render service: confirm HTTPS-only access (Render provides TLS by default).
2. PostgreSQL (Render): ensure `sslmode=require` is enforced by provider.
3. Redis: confirm TLS enabled (if using managed Redis).
4. Object storage (S3-compatible): HTTPS-only endpoints.
5. External APIs (Meta, Google, Microsoft, Stripe, Twilio): HTTPS enforced by default.

## At Rest
1. Render PostgreSQL: verify encryption at rest enabled in Render dashboard.
2. Redis: verify encryption at rest in provider dashboard.
3. Object storage: verify server-side encryption enabled (SSE-S3 or SSE-KMS).
4. Pinecone: verify encryption at rest in Pinecone settings.

## Evidence
- Capture screenshots of provider settings and store links in SOC2 tracker.

# Key Rotation Runbook

## Purpose
Rotate encryption keys and secrets without service interruption.

## Scope
- KMS CMKs/KEKs
- OAuth client secrets
- API keys (LLM and external providers)
- Webhook shared secrets
- mTLS cert materials

## Procedure
1. Schedule low-traffic rotation window.
2. Create new key material in KMS/Secrets Manager.
3. Update secret versions with staged labels (`AWSPENDING` -> `AWSCURRENT`).
4. Restart services with rolling strategy.
5. Re-encrypt sensitive records if required (OAuth token envelope data).
6. Validate integrations.
   - token refresh
   - outbound provider calls
   - webhook verification
7. Keep previous key active for 24 hours.
8. Disable old key after validation window.

## Verification Checklist
- No spike in `AUTH_EXPIRED` / `AUTH_REVOKED` errors.
- Secrets fetch latency within baseline.
- No failed decrypt events.

## Rollback
- Restore previous key alias/version.
- Roll services back to previous secret reference.
- Re-run integration smoke tests.

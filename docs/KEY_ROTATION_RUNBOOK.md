# Key Rotation Runbook

## What Gets Rotated
- `JWT_SECRET`
- `STATE_SIGNING_SECRET`
- `TOKEN_ENCRYPTION_KEY` / `PII_ENCRYPTION_KEYS`
- Provider secrets (Meta, Google, Microsoft, Stripe, Twilio, etc.)

## Standard Rotation (Quarterly)
1. Generate new secret (store securely).
2. Add new secret alongside old (where multi‑key supported).
3. Deploy with both keys enabled.
4. Re‑encrypt or re‑issue tokens as needed.
5. Remove old secret after confirmation window.

## Emergency Rotation (Compromise)
1. Revoke and replace the compromised secret immediately.
2. Invalidate all affected tokens/sessions.
3. Notify stakeholders and document incident.
4. Audit access logs for misuse.

## Evidence
- Record rotation date and affected systems in the SOC2 tracker.

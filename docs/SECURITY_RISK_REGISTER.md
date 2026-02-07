# Security Risk Register (Initial)

| Risk | Impact | Likelihood | Mitigation | Owner |
|------|--------|------------|------------|-------|
| Webhook spoofing | High | Medium | Signature verification + IP allowlist | Security |
| PII leakage in logs | High | Medium | PII masking + structured logging | Platform |
| Account takeover | High | Medium | JWT + signed headers + rate limiting | Platform |
| Prompt injection | Medium | Medium | Tool allowlist + approval flow | AI |

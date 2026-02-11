# Security Risk Register

| Risk | Impact | Likelihood | Mitigation | Owner |
|------|--------|------------|------------|-------|
| Webhook spoofing | High | Medium | Signature verification + dedupe; optional IP allowlist | Security |
| PII leakage in logs | High | Medium | PII masking, structured logging, audit logs | Platform |
| Account takeover | High | Medium | JWT + signed headers + rate limiting + MFA for admins | Platform |
| Prompt injection | Medium | Medium | Tool allowlist + approval flow + guardrails | AI |
| Token replay | Medium | Medium | Short TTLs, replay windows, signature checks | Platform |
| Data exfiltration via mis-scoped OAuth | High | Low | Minimum scopes + consent logging + revocation | Platform |
| Payment abuse | High | Low | Spending limits + approvals + audit logs | Platform |
| Insider access misuse | High | Low | Least privilege + access reviews + audit logs | Security |

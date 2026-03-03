# Security Architecture

## Trust Boundaries
1. Internet -> Gateway
   - WAF, IP filtering, webhook signature verification, rate limiting.
2. Gateway -> Brain
   - mTLS, schema validation, authenticated service identity.
3. Brain -> Hands
   - mTLS, OPA authorization, allowlisted skill routing.
4. Hands -> External APIs
   - OAuth/API keys from Secrets Manager, least privilege scopes.
5. Services -> Data stores
   - TLS in transit, encrypted at rest, role-restricted DB access.
6. Services -> Temporal
   - mTLS and namespace isolation.

## Encryption Standards
- DB/S3: AES-256 with KMS-managed keys.
- OAuth tokens: AES-256-GCM envelope encryption.
- Transit: TLS 1.3 external, mTLS internal.
- Webhook auth: HMAC-SHA256 signatures.

## Identity and Access
- Role model: free/pro/enterprise/admin/service.
- Least privilege enforced via OPA policies and RBAC scopes.
- Secrets never hardcoded; centralized in Secrets Manager.

## Threat Model and Mitigations
- Webhook spoofing -> signature verification + replay protection.
- Token theft -> encrypted token storage + key rotation.
- Prompt injection -> input sanitization + output schema checks.
- Skill impersonation -> registry IDs + contract validation.
- Data exfiltration -> PII controls + scoped context injection.
- Supply chain attack -> signed artifacts + SBOM + vulnerability scans.

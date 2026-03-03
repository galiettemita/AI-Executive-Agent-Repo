# Skill Onboarding Runbook

## Purpose
Onboard a new skill adapter with deterministic behavior, policy compliance, and release safety.

## Development Steps
1. Create adapter folder using canonical layout.
2. Implement `ISkillAdapter` with strict input/output schema validation.
3. Implement `client.ts` with retry/circuit-breaker behavior.
4. Register skill in `skills.registry` seed/migration.
5. Add disambiguation rule updates if overlap exists.

## Test Requirements
- Unit test with mocked external responses.
- Integration test against sandbox/staging provider.
- Policy checks for tier access and budget enforcement.
- Failure tests: auth expiry, rate limit, timeout, external 5xx.

## Security Checks
- No secret logging.
- OAuth scopes minimal and documented.
- Financial/critical actions require confirmation gates.

## Release Steps
1. Deploy behind feature flag.
2. Enable for internal users only.
3. Observe error rate, latency, cost, and circuit state for 24h.
4. Gradually expand rollout by tier.

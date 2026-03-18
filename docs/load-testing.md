# Load Testing & Production Gate

## Overview
Load tests run automatically after every successful staging deployment via
`.github/workflows/load-tests.yml`.

## Thresholds (see config/load-test-thresholds.json)
| Test   | VUs | Duration | p99 threshold | Error rate |
|--------|-----|----------|--------------|------------|
| Smoke  | 1   | 1m       | < 2000ms     | < 1%       |
| Load   | 50  | 5m       | < 5000ms     | < 2%       |

## Production Promotion Gate
ArgoCD is configured to require the `production-gate` job in the `Load Tests` workflow
to succeed before promoting the staging release to production.

ArgoCD configuration path: `infra/argocd/apps/brevio-production.yaml`

## Required GitHub Secrets
- `STAGING_URL` — Base URL of the staging gateway
- `STAGING_BRAIN_URL` — Base URL of the staging brain service
- `LOAD_TEST_WORKSPACE_ID` — Dedicated test workspace ID
- `PROMETHEUS_REMOTE_WRITE_URL` — Remote write endpoint for Prometheus metrics

## Running Manually
```bash
# From GitHub Actions UI: Run workflow > load-tests.yml
# Or locally with k6 installed:
k6 run tests/load/k6-smoke.js --env GATEWAY_URL=https://staging.brevio.ai --env BRAIN_URL=https://staging-brain.brevio.ai
k6 run tests/load/k6-load-test.js --env GATEWAY_URL=https://staging.brevio.ai --env BRAIN_URL=https://staging-brain.brevio.ai
```

## Additional Test Scripts
- `k6_interactive_turn.js` — Tier-based interactive turn latency test (T1/T2/T3)
- `k6_load_shedding.js` — Load shedding behavior test across degradation tiers (D0–D5)
- `k6_streaming_first_byte.js` — Streaming first-byte latency test (p95 < 500ms)

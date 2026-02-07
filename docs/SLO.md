# Service Level Objectives (SLOs)

## API Availability
- Target: 99.9% monthly availability for core API endpoints
- Error budget: 43 minutes/month

## Latency
- P50: < 200ms
- P95: < 600ms
- P99: < 1500ms

## Background Jobs
- Daily brief and scheduled jobs: 99% within SLA window
- Retry policy: exponential backoff with max 5 retries

## Incident Response
- P0: 15 min response, 2 hr mitigation
- P1: 1 hr response, 1 business day mitigation

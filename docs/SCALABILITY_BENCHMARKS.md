# Scalability Benchmarks

These benchmarks reflect US-only launch, 10,000 users in Year 1, and a $29/mo plan.

## Availability and Latency
- Availability target: 99.9% monthly.
- Latency targets:
  - Chat/assist: P95 < 2s
  - Heavy tasks: P95 < 5s

## Load Targets (Year 1)
- Peak concurrent users: 500
- Peak requests per second (API): 50 RPS
- Webhook burst handling: 200 requests/minute

## Background Jobs
- Email monitoring: 1-2 runs/hour per active user.
- Notification delivery: within 5 minutes of queue.
- Rotation reminders: daily batch, 10k users within 15 minutes.

## Storage and Data
- Object storage: plan for 1-2 GB/user for files/photos (user-controlled).
- Vector store: embeddings for files/photos; batch ingestion.

## Cost Benchmarks
- Target infrastructure + AI cost per user: <= $5/month.
- If costs exceed target, reduce model usage, increase caching, or limit heavy tasks.

## Scaling Triggers
- CPU > 70% for 15 minutes: scale web replicas.
- Queue length > 5,000: scale worker replicas.
- DB CPU > 70%: scale DB or add read replica.

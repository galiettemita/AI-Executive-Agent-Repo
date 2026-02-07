# Message Queue and Workers

Celery is configured as the background worker system.

## Broker
- Use Redis (recommended) or another broker via CELERY_BROKER_URL

## Worker
- Start worker:
  celery -A app.core.celery_app.celery_app worker --loglevel=info

## Beat
- Start scheduler (later phases):
  celery -A app.core.celery_app.celery_app beat --loglevel=info

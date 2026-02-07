# Local Development Setup

This project is designed for a production-grade stack. Local development should mirror production as closely as possible (PostgreSQL + Redis).

## Prerequisites
- Docker Desktop
- Python 3.11+

## 1) Start local infrastructure
```bash
docker compose up -d
```

## 2) Configure environment
Create a local `.env` file with the following baseline values:
```bash
ENV=dev
DATABASE_URL=postgresql+psycopg://executive:executive@localhost:5432/executive_ai_agent
REDIS_URL=redis://localhost:6379/0
JWT_SECRET=dev_only_change_me
OPENAI_API_KEY=<your-openai-key>
PII_ENCRYPTION_KEY=<your-fernet-key>
```

Generate a Fernet key:
```bash
python3 - <<'PY'
from cryptography.fernet import Fernet
print(Fernet.generate_key().decode())
PY
```

## 3) Install dependencies
```bash
python3 -m pip install -r requirements.txt
```

## 4) Run migrations
```bash
alembic upgrade head
```

## 5) Start the API
```bash
uvicorn app.main:app --reload
```

## Notes
- In `staging` or `production`, the app will refuse to start if `DATABASE_URL` points to SQLite or if critical keys are missing.
- Keep local infra running with `docker compose down` when finished.

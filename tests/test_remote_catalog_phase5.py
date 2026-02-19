from __future__ import annotations

from sqlalchemy import text

from app.core.config import settings
from app.db.database import SessionLocal
from app.services import scheduler
from app.services import remote_catalog


def test_sync_remote_catalog_upserts_entries(monkeypatch):
    monkeypatch.setattr(settings, "PROVISIONING_REMOTE_SYNC_ENABLED", True)
    monkeypatch.setattr(settings, "REMOTE_CATALOG_API_URL", "https://catalog.example.com")
    monkeypatch.setattr(
        remote_catalog,
        "fetch_remote_catalog_snapshot",
        lambda: [
            {
                "server_id": "remote-alpha-mcp",
                "display_name": "Remote Alpha",
                "description": "Alpha capability",
                "auth_type": "oauth2",
                "min_plan": "professional",
                "setup_seconds": 30,
                "capabilities": ["alpha"],
                "keywords": ["alpha"],
                "source": "remote",
            },
            {
                "server_id": "remote-beta-mcp",
                "display_name": "Remote Beta",
                "description": "Beta capability",
                "auth_type": "api_key",
                "min_plan": "professional",
                "setup_seconds": 45,
                "capabilities": ["beta"],
                "keywords": ["beta"],
                "source": "remote",
            },
        ],
    )

    db = SessionLocal()
    try:
        result = remote_catalog.sync_remote_catalog(db)
        assert result.get("ok") is True
        assert int(result.get("entries_fetched") or 0) == 2

        count = db.execute(
            text("select count(*) from server_catalog where source = 'remote' and server_id like 'remote-%'")
        ).scalar()
        assert int(count or 0) >= 2
    finally:
        db.close()


def test_scheduler_includes_remote_catalog_sync_job_when_enabled(monkeypatch):
    monkeypatch.setattr(settings, "PROVISIONING_REMOTE_SYNC_ENABLED", True)
    sched = scheduler.setup_scheduler()
    try:
        job = sched.get_job("remote_catalog_daily_sync_job")
        assert job is not None
    finally:
        try:
            sched.shutdown(wait=False)
        except Exception:
            pass

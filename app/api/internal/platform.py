from __future__ import annotations

import os
from datetime import datetime
from uuid import uuid4

from fastapi import APIRouter, BackgroundTasks, Header, HTTPException
from pydantic import BaseModel
from fastapi.responses import JSONResponse
from sqlalchemy import text

from app.blueprint.contracts import InboundMessage
from app.blueprint.bones import refresh_bones_catalog
from app.blueprint.embedding_audit import run_embedding_reembed_audit, run_embedding_reembed_audit_all_users
from app.blueprint.muscles import capture_muscles_snapshot
from app.blueprint.progress import get_run_result
from app.core.config import settings
from app.core.metrics import metrics_response
from app.core.redis import get_redis
from app.db.database import SessionLocal
from app.api.routes.v1_gateway import enqueue_inbound_message
from app.blueprint.knowledge_review import run_nightly_consolidation, run_weekly_self_review
from app.blueprint.mcp.hub import get_mcp_client_hub
from app.services.proactive_rules import run_due_rules
from app.services.scheduler import run_due_research
from app.services.experiments import assign_variant, create_experiment, list_experiments


router = APIRouter(prefix="/internal", tags=["internal-platform"])


class KnowledgeReviewRequest(BaseModel):
    user_id: str


class TriggerFireRequest(BaseModel):
    include_proactive: bool = True
    include_research: bool = True


class CacheFlushRequest(BaseModel):
    patterns: list[str] | None = None


class UserScopedRequest(BaseModel):
    user_id: str


class ExperimentCreateRequest(BaseModel):
    name: str
    description: str = ""
    status: str = "draft"
    prompt_group: str | None = None
    allocation: dict[str, int] | None = None
    config: dict[str, object] | None = None
    created_by: str | None = None


def _db_status() -> str:
    try:
        from app.db.database import SessionLocal

        db = SessionLocal()
        try:
            db.execute(text("select 1"))
            return "ok"
        finally:
            db.close()
    except Exception:
        return "unavailable"


def _redis_status() -> str:
    if not settings.REDIS_URL:
        return "not_configured"
    try:
        from app.core.redis import redis_ping

        redis_ping()
        return "ok"
    except Exception:
        return "unavailable"


@router.get("/health")
def internal_health():
    return {
        "status": "ok",
        "env": settings.ENV,
        "time": datetime.utcnow().isoformat(),
        "version": settings.APP_VERSION
        or os.getenv("RENDER_GIT_COMMIT", "")
        or os.getenv("VERCEL_GIT_COMMIT_SHA", ""),
    }


@router.get("/health/deep")
def internal_health_deep():
    db = _db_status()
    redis = _redis_status()
    providers = {
        "openai": bool(settings.OPENAI_API_KEY),
        "anthropic": bool(settings.ANTHROPIC_API_KEY),
        "google_ai": bool(settings.GOOGLE_AI_API_KEY),
    }
    all_ok = db == "ok" and redis in {"ok", "not_configured"}
    return JSONResponse(
        status_code=200 if all_ok else 503,
        content={
            "status": "ready" if all_ok else "degraded",
            "checks": {
                "database": db,
                "redis": redis,
                "providers_configured": providers,
            },
        },
    )


@router.get("/metrics")
def internal_metrics(authorization: str | None = Header(default=None)):
    if settings.ENV in ("production", "staging") and settings.METRICS_TOKEN:
        if not authorization or not authorization.startswith("Bearer "):
            raise HTTPException(status_code=401, detail="Missing metrics token")
        token = authorization.split(" ", 1)[1].strip()
        if token != settings.METRICS_TOKEN:
            raise HTTPException(status_code=403, detail="Invalid metrics token")
    return metrics_response()


@router.post("/runs/{run_id}/replay")
def internal_replay_run(run_id: str, background_tasks: BackgroundTasks):
    result = get_run_result(run_id)
    if not result:
        raise HTTPException(status_code=404, detail="Run result not found")
    inbound_payload = result.get("inbound")
    if not isinstance(inbound_payload, dict):
        raise HTTPException(status_code=400, detail="Replay payload unavailable for this run")
    try:
        inbound = InboundMessage.model_validate(inbound_payload)
    except Exception as exc:
        raise HTTPException(status_code=400, detail=f"Invalid replay payload: {exc}")
    replay_run_id = str(uuid4())
    replay_inbound = inbound.model_copy(update={"run_id": replay_run_id})
    enqueue_inbound_message(background_tasks=background_tasks, inbound=replay_inbound, run_id=replay_run_id)
    return {"ok": True, "replayed_from": run_id, "run_id": replay_run_id}


@router.post("/cache/flush")
def internal_cache_flush(payload: CacheFlushRequest):
    patterns = payload.patterns or [
        "bp:knowledge:latest:*",
        "bp:progress:*",
        "bp:run-result:*",
        "bp:llm:health:*",
        "mcp:cost:*",
    ]
    deleted = 0
    redis_client = get_redis()
    if redis_client:
        for pattern in patterns:
            for key in redis_client.scan_iter(match=pattern, count=200):
                try:
                    deleted += int(redis_client.delete(key) or 0)
                except Exception:
                    continue

    mcp_cache = get_mcp_client_hub().flush_caches()
    return {
        "ok": True,
        "redis_keys_deleted": deleted,
        "patterns": patterns,
        "mcp_cache": mcp_cache,
    }


@router.post("/triggers/fire")
def internal_fire_triggers(payload: TriggerFireRequest):
    proactive_result: dict | None = None
    if payload.include_proactive:
        db = SessionLocal()
        try:
            proactive_result = run_due_rules(db)
            db.commit()
        except Exception as exc:
            db.rollback()
            proactive_result = {"ok": False, "error": str(exc)}
        finally:
            db.close()

    research_result: dict | None = None
    if payload.include_research:
        research_result = run_due_research()

    return {
        "ok": True,
        "proactive": proactive_result,
        "research": research_result,
    }


@router.post("/bones/refresh")
def internal_bones_refresh(payload: UserScopedRequest):
    db = SessionLocal()
    try:
        result = refresh_bones_catalog(db, user_id=payload.user_id)
        db.commit()
        return {"ok": True, **result}
    except Exception as exc:
        db.rollback()
        raise HTTPException(status_code=400, detail=f"Bones refresh failed: {exc}")
    finally:
        db.close()


@router.post("/muscles/snapshot")
def internal_muscles_snapshot(payload: UserScopedRequest):
    db = SessionLocal()
    try:
        result = capture_muscles_snapshot(db, user_id=payload.user_id)
        db.commit()
        return {"ok": True, **result}
    except Exception as exc:
        db.rollback()
        raise HTTPException(status_code=400, detail=f"Muscles snapshot failed: {exc}")
    finally:
        db.close()


@router.post("/embeddings/reembed")
def internal_embedding_reembed(payload: UserScopedRequest | None = None):
    db = SessionLocal()
    try:
        if payload and payload.user_id:
            result = run_embedding_reembed_audit(db, user_id=payload.user_id)
        else:
            result = run_embedding_reembed_audit_all_users(db)
        db.commit()
        return {"ok": True, **result}
    except Exception as exc:
        db.rollback()
        raise HTTPException(status_code=400, detail=f"Embedding audit failed: {exc}")
    finally:
        db.close()


@router.post("/knowledge/consolidate")
def internal_knowledge_consolidate(payload: KnowledgeReviewRequest):
    db = SessionLocal()
    try:
        result = run_nightly_consolidation(db, user_id=payload.user_id)
        db.commit()
        return {"ok": True, **result}
    except Exception as exc:
        db.rollback()
        raise HTTPException(status_code=400, detail=f"Consolidation failed: {exc}")
    finally:
        db.close()


@router.post("/knowledge/review")
def internal_knowledge_review(payload: KnowledgeReviewRequest):
    db = SessionLocal()
    try:
        result = run_weekly_self_review(db, user_id=payload.user_id)
        db.commit()
        return {"ok": True, **result}
    except Exception as exc:
        db.rollback()
        raise HTTPException(status_code=400, detail=f"Self review failed: {exc}")
    finally:
        db.close()


@router.get("/experiments")
def internal_list_experiments(
    status: str | None = None,
    limit: int = 100,
    user_id: str | None = None,
):
    db = SessionLocal()
    try:
        rows = list_experiments(db, status=status, limit=limit)
        if user_id:
            for row in rows:
                allocation = row.get("allocation") if isinstance(row.get("allocation"), dict) else {}
                row["assigned_variant"] = assign_variant(
                    experiment_id=str(row.get("id") or ""),
                    user_id=user_id,
                    allocation=allocation if isinstance(allocation, dict) else {},
                )
        return {"ok": True, "items": rows}
    except Exception as exc:
        raise HTTPException(status_code=400, detail=f"Experiment lookup failed: {exc}")
    finally:
        db.close()


@router.post("/experiments")
def internal_create_experiment(payload: ExperimentCreateRequest):
    db = SessionLocal()
    try:
        row = create_experiment(
            db,
            name=payload.name,
            description=payload.description,
            status=payload.status,
            prompt_group=payload.prompt_group,
            allocation=payload.allocation,
            config=payload.config,
            created_by=payload.created_by,
        )
        return {"ok": True, "experiment": row}
    except ValueError as exc:
        raise HTTPException(status_code=400, detail=str(exc))
    except Exception as exc:
        raise HTTPException(status_code=400, detail=f"Experiment create failed: {exc}")
    finally:
        db.close()

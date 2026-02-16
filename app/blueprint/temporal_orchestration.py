from __future__ import annotations

import asyncio
import logging
from dataclasses import dataclass
from datetime import timedelta
from typing import Any

from app.core.config import settings

logger = logging.getLogger(__name__)

try:  # Optional dependency; feature is gated by env flags.
    from temporalio.client import Client
except Exception:  # pragma: no cover - exercised when temporalio isn't installed
    Client = None


@dataclass(frozen=True)
class PlannedSubtask:
    task: str
    tier: int
    retry_limit: int
    critical: bool = False

    def to_dict(self) -> dict[str, Any]:
        return {
            "task": self.task,
            "tier": self.tier,
            "retry_limit": self.retry_limit,
            "critical": self.critical,
        }


def infer_subtask_tier(task: str) -> int:
    text = (task or "").strip().lower()
    if any(k in text for k in ("send", "book", "buy", "purchase", "delete", "pay", "wire", "transfer")):
        return 3
    if any(k in text for k in ("search", "find", "look up", "research", "summarize", "analyze", "list")):
        return 2
    return 1


def _retry_limit_for_task(task: str, tier: int) -> int:
    text = (task or "").lower()
    if any(k in text for k in ("send", "book", "buy", "purchase", "delete", "pay", "wire")):
        return 1
    if tier >= 2:
        return 2
    return 1


def _local_orchestration_plan(subtasks: list[str]) -> list[dict[str, Any]]:
    out: list[dict[str, Any]] = []
    for task in subtasks:
        tier = infer_subtask_tier(task)
        out.append(
            PlannedSubtask(
                task=task,
                tier=tier,
                retry_limit=_retry_limit_for_task(task, tier),
                critical=tier >= 3,
            ).to_dict()
        )
    return out


async def _run_temporal_plan(
    *,
    run_id: str | None,
    user_id: str | None,
    subtasks: list[str],
) -> list[dict[str, Any]]:
    if Client is None:
        raise RuntimeError("temporalio client not installed")
    host = (settings.TEMPORAL_HOST or "").strip()
    if not host:
        raise RuntimeError("TEMPORAL_HOST not configured")

    client = await Client.connect(host, namespace=settings.TEMPORAL_NAMESPACE or "default")
    workflow_id = f"tier3-plan-{run_id or 'ad-hoc'}"
    result = await client.execute_workflow(
        settings.TEMPORAL_TIER3_WORKFLOW_NAME or "Tier3PlannerWorkflow",
        {
            "run_id": run_id,
            "user_id": user_id,
            "subtasks": subtasks,
        },
        id=workflow_id,
        task_queue=settings.TEMPORAL_TASK_QUEUE_TIER3 or "executive-os-tier3",
        run_timeout=timedelta(seconds=max(30, int(settings.TEMPORAL_WORKFLOW_TIMEOUT_S))),
    )
    if isinstance(result, list):
        normalized: list[dict[str, Any]] = []
        for item in result:
            if not isinstance(item, dict):
                continue
            task = str(item.get("task") or "").strip()
            if not task:
                continue
            tier = int(item.get("tier") or infer_subtask_tier(task))
            retry_limit = int(item.get("retry_limit") or _retry_limit_for_task(task, tier))
            normalized.append(
                {
                    "task": task,
                    "tier": max(1, min(3, tier)),
                    "retry_limit": max(1, min(3, retry_limit)),
                    "critical": bool(item.get("critical", tier >= 3)),
                }
            )
        if normalized:
            return normalized
    raise RuntimeError("Temporal workflow returned invalid plan")


def orchestrate_tier3_plan(
    *,
    run_id: str | None,
    user_id: str | None,
    subtasks: list[str],
) -> tuple[list[dict[str, Any]], dict[str, Any]]:
    cleaned = [str(s).strip() for s in subtasks if str(s).strip()]
    if not cleaned:
        return [], {"engine": "none"}

    if not settings.TEMPORAL_ENABLED:
        return _local_orchestration_plan(cleaned), {"engine": "local", "reason": "temporal_disabled"}

    if Client is None:
        return _local_orchestration_plan(cleaned), {"engine": "local", "reason": "temporal_client_missing"}

    try:
        running_loop = asyncio.get_running_loop()
        if running_loop and running_loop.is_running():
            return _local_orchestration_plan(cleaned), {"engine": "local", "reason": "event_loop_already_running"}
    except RuntimeError:
        pass

    try:
        planned = asyncio.run(
            _run_temporal_plan(
                run_id=run_id,
                user_id=user_id,
                subtasks=cleaned,
            )
        )
        return planned, {"engine": "temporal", "task_queue": settings.TEMPORAL_TASK_QUEUE_TIER3}
    except Exception as exc:
        logger.warning("Temporal orchestration fallback to local: %s", exc)
        return _local_orchestration_plan(cleaned), {"engine": "local", "reason": f"temporal_error:{exc.__class__.__name__}"}


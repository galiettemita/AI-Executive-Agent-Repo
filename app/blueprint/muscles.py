from __future__ import annotations

from datetime import datetime
from typing import Any

from sqlalchemy import text
from sqlalchemy.orm import Session

from app.blueprint.contracts import LLMRequest
from app.blueprint.knowledge_files import get_latest_knowledge_file, put_knowledge_file_version
from app.blueprint.llm.router import get_llm_router
from app.services.circuit_breaker import get_all_circuit_breakers


def _collect_route_samples() -> list[dict[str, Any]]:
    router = get_llm_router()
    tasks = [
        "intent_classification",
        "single_tool_call",
        "email_drafting",
        "complex_reasoning",
        "knowledge_extraction",
        "general",
    ]
    out: list[dict[str, Any]] = []
    for task in tasks:
        route = router.select_route(
            LLMRequest(
                messages=[{"role": "user", "content": f"Route probe: {task}"}],
                task_type=task,
                max_tokens=64,
                temperature=0,
            )
        )
        out.append(route)
    return out


def _collect_provider_health() -> list[dict[str, Any]]:
    router = get_llm_router()
    health_map = router.all_provider_health(force_refresh=False)
    out: list[dict[str, Any]] = []
    for provider, item in health_map.items():
        out.append(
            {
                "provider": provider,
                "healthy": bool(item.is_healthy),
                "latency_p95_ms": int(item.latency_p95_ms or 0),
                "error_rate_1h": float(item.error_rate_1h or 0.0),
            }
        )
    return sorted(out, key=lambda row: row["provider"])


def _collect_mcp_health(db: Session) -> list[dict[str, Any]]:
    try:
        rows = db.execute(
            text(
                """
                select server_id, display_name, state, health_status, total_calls, total_errors, total_cost_cents
                from mcp_servers
                order by display_name asc
                """
            )
        ).mappings().all()
    except Exception:
        return []
    out: list[dict[str, Any]] = []
    for row in rows:
        out.append(
            {
                "server_id": str(row.get("server_id") or ""),
                "display_name": str(row.get("display_name") or row.get("server_id") or ""),
                "state": str(row.get("state") or "registered"),
                "health_status": str(row.get("health_status") or "unknown"),
                "total_calls": int(row.get("total_calls") or 0),
                "total_errors": int(row.get("total_errors") or 0),
                "total_cost_cents": float(row.get("total_cost_cents") or 0.0),
            }
        )
    return out


def _render_heartbeat_section(snapshot: dict[str, Any]) -> str:
    lines = [
        "## Muscles Snapshot",
        f"_Generated: {snapshot['generated_at']}_",
        "",
        "### Provider Health",
    ]
    providers = snapshot.get("provider_health") or []
    if providers:
        for item in providers:
            lines.append(
                f"- {item['provider']}: healthy={item['healthy']} "
                f"latency_p95={item['latency_p95_ms']}ms error_rate_1h={item['error_rate_1h']}"
            )
    else:
        lines.append("- No provider health data.")

    lines.extend(["", "### Route Matrix"])
    routes = snapshot.get("route_samples") or []
    if routes:
        for route in routes:
            lines.append(
                f"- {route.get('task_type')}: {route.get('selected_provider')}:{route.get('selected_model')}"
            )
    else:
        lines.append("- No route samples.")

    lines.extend(["", "### Circuit Breakers"])
    breakers = snapshot.get("circuit_breakers") or {}
    if breakers:
        for name in sorted(breakers.keys()):
            item = breakers.get(name) or {}
            lines.append(
                f"- {name}: state={item.get('state')} failures={item.get('failure_count')} "
                f"successes={item.get('success_count')}"
            )
    else:
        lines.append("- No circuit breaker state.")

    lines.extend(["", "### MCP Health"])
    mcp_health = snapshot.get("mcp_health") or []
    if mcp_health:
        for item in mcp_health:
            lines.append(
                f"- {item['display_name']} ({item['server_id']}): state={item['state']} "
                f"health={item['health_status']} calls={item['total_calls']} errors={item['total_errors']}"
            )
    else:
        lines.append("- No MCP servers registered.")
    return "\n".join(lines).strip()


def capture_muscles_snapshot(db: Session, *, user_id: str) -> dict[str, Any]:
    snapshot = {
        "generated_at": datetime.utcnow().isoformat(),
        "provider_health": _collect_provider_health(),
        "route_samples": _collect_route_samples(),
        "circuit_breakers": get_all_circuit_breakers(),
        "mcp_health": _collect_mcp_health(db),
    }

    latest = get_latest_knowledge_file(db, user_id=user_id, file_path="HEARTBEAT.md")
    current = str((latest or {}).get("content") or "").strip()
    muscles_section = _render_heartbeat_section(snapshot)
    marker = "## Muscles Snapshot"
    if marker in current:
        merged = current.split(marker)[0].rstrip() + "\n\n" + muscles_section
    else:
        merged = (current + "\n\n" + muscles_section).strip() if current else "\n".join(["# HEARTBEAT.md", "", muscles_section])
    put_knowledge_file_version(
        db,
        user_id=user_id,
        file_path="HEARTBEAT.md",
        content=merged.strip(),
        metadata={"source": "muscles_snapshot"},
    )
    return {"ok": True, "snapshot": snapshot}

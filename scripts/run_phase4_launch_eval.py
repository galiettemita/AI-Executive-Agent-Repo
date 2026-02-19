from __future__ import annotations

import asyncio
from datetime import datetime, timezone
from pathlib import Path

from app.blueprint.mcp.contracts import MCPRunContext
from app.blueprint.mcp.hub import get_mcp_client_hub
from app.blueprint.mcp.mock_servers import mock_tools_for
from app.blueprint.mcp.wave1_catalog import bootstrap_wave1_servers
from app.blueprint.mcp.wave2_catalog import bootstrap_wave2_servers
from app.blueprint.mcp.wave3_catalog import bootstrap_wave3_servers
from app.blueprint.mcp.wave4_catalog import bootstrap_wave4_servers
from app.db.database import SessionLocal

REPORT_PATH = Path("docs/reports/phase4_launch_eval.md")


async def _bootstrap_all(user_id: str) -> list[str]:
    db = SessionLocal()
    try:
        summaries = [
            await bootstrap_wave1_servers(db, user_id=user_id, transport_mode="mock", connect=True),
            await bootstrap_wave2_servers(db, user_id=user_id, transport_mode="mock", connect=True),
            await bootstrap_wave3_servers(db, user_id=user_id, transport_mode="mock", connect=True),
            await bootstrap_wave4_servers(db, user_id=user_id, transport_mode="mock", connect=True),
        ]
    finally:
        db.close()

    servers: list[str] = []
    for summary in summaries:
        for item in summary.get("items") or []:
            server_id = str(item.get("server_id") or "")
            if server_id:
                servers.append(server_id)
    return servers


async def run() -> dict[str, object]:
    user_id = "phase4-launch-eval-user"
    servers = await _bootstrap_all(user_id)
    hub = get_mcp_client_hub()
    rows: list[dict[str, object]] = []

    for idx, server_id in enumerate(servers):
        tools = mock_tools_for(server_id)
        tool_name = str((tools[0].name if tools else "health.check") or "health.check")
        db = SessionLocal()
        try:
            result = await hub.call_tool(
                db,
                server_id=server_id,
                tool_name=tool_name,
                arguments={"sample": True},
                run_context=MCPRunContext(
                    run_id=f"phase4-launch-eval-{idx}",
                    user_id=user_id,
                    provenance="user_direct",
                ),
            )
            rows.append(
                {
                    "server_id": server_id,
                    "ok": not bool(result.is_error),
                    "latency_ms": int(result.latency_ms or 0),
                }
            )
        except Exception as exc:  # pragma: no cover - script mode
            rows.append({"server_id": server_id, "ok": False, "latency_ms": 0, "error": str(exc)})
        finally:
            db.close()

    success = sum(1 for row in rows if bool(row.get("ok")))
    return {
        "generated_at": datetime.now(timezone.utc).isoformat(),
        "server_count": len(rows),
        "success_count": success,
        "failure_count": len(rows) - success,
        "rows": rows,
    }


def _write_report(payload: dict[str, object]) -> None:
    rows = payload.get("rows") or []
    lines = [
        "# Phase 4 Launch Eval Baseline (Waves 1-4)",
        "",
        f"Generated at: {payload.get('generated_at')}",
        f"Servers evaluated: {payload.get('server_count')}",
        f"Successful calls: {payload.get('success_count')}",
        f"Failed calls: {payload.get('failure_count')}",
        "",
        "| Server | OK | Latency (ms) |",
        "|---|---:|---:|",
    ]
    for row in rows:
        lines.append(
            f"| `{row.get('server_id')}` | {'yes' if row.get('ok') else 'no'} | {int(row.get('latency_ms') or 0)} |"
        )
    REPORT_PATH.parent.mkdir(parents=True, exist_ok=True)
    REPORT_PATH.write_text("\n".join(lines) + "\n", encoding="utf-8")


def main() -> None:
    payload = asyncio.run(run())
    _write_report(payload)
    print(str(REPORT_PATH))


if __name__ == "__main__":
    main()

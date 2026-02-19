from __future__ import annotations

import asyncio
import json
import random
import time
from datetime import datetime, timezone
from pathlib import Path

from app.blueprint.mcp.contracts import MCPRunContext
from app.blueprint.mcp.hub import get_mcp_client_hub
from app.blueprint.mcp.mock_servers import mock_tools_for
from app.blueprint.mcp.wave3_catalog import bootstrap_wave3_servers
from app.db.database import SessionLocal

REPORT_PATH = Path("docs/reports/phase4_wave3_load_failover.json")


async def _invoke_once(*, server_id: str, tool_name: str, user_id: str, run_id: str, idx: int) -> dict[str, object]:
    db = SessionLocal()
    try:
        hub = get_mcp_client_hub()
        started = time.perf_counter()
        result = await hub.call_tool(
            db,
            server_id=server_id,
            tool_name=tool_name,
            arguments={"idx": idx},
            run_context=MCPRunContext(run_id=run_id, user_id=user_id, provenance="user_direct"),
        )
        return {
            "ok": not bool(result.is_error),
            "server_id": server_id,
            "latency_ms": int((time.perf_counter() - started) * 1000),
        }
    except Exception as exc:  # pragma: no cover - script path
        return {"ok": False, "server_id": server_id, "error": str(exc)}
    finally:
        db.close()


async def run() -> dict[str, object]:
    user_id = "phase4-wave3-load-user"
    db = SessionLocal()
    try:
        summary = await bootstrap_wave3_servers(db, user_id=user_id, transport_mode="mock", connect=True)
    finally:
        db.close()

    servers = [str(item.get("server_id") or "") for item in (summary.get("items") or []) if str(item.get("server_id") or "")]
    if not servers:
        raise RuntimeError("No Wave 3 servers available for load test")
    tool_map: dict[str, str] = {}
    for server_id in servers:
        tools = mock_tools_for(server_id)
        tool_map[server_id] = str((tools[0].name if tools else "health.check") or "health.check")

    run_id = "phase4-wave3-load"
    tasks = []
    for i in range(100):
        server_id = servers[i % len(servers)]
        tasks.append(
            _invoke_once(
                server_id=server_id,
                tool_name=tool_map.get(server_id) or "health.check",
                user_id=user_id,
                run_id=run_id,
                idx=i,
            )
        )

    started = time.perf_counter()
    results = await asyncio.gather(*tasks)
    elapsed_ms = int((time.perf_counter() - started) * 1000)
    success = sum(1 for row in results if bool(row.get("ok")))

    # Failover simulation: disconnect 5 random servers, then verify calls still recover.
    hub = get_mcp_client_hub()
    failover_targets = random.sample(servers, k=min(5, len(servers)))
    failover_results: list[dict[str, object]] = []
    for idx, server_id in enumerate(failover_targets):
        db_disc = SessionLocal()
        try:
            await hub.disconnect_server(db_disc, user_id=user_id, server_id=server_id)
        finally:
            db_disc.close()
        after = await _invoke_once(
            server_id=server_id,
            tool_name=tool_map.get(server_id) or "health.check",
            user_id=user_id,
            run_id="phase4-wave3-failover",
            idx=idx,
        )
        failover_results.append(after)

    failover_success = sum(1 for row in failover_results if bool(row.get("ok")))

    payload = {
        "generated_at": datetime.now(timezone.utc).isoformat(),
        "wave": "wave3",
        "concurrent_calls": 100,
        "servers": servers,
        "total_success": success,
        "total_failures": len(results) - success,
        "elapsed_ms": elapsed_ms,
        "avg_latency_ms": round(sum(int(row.get("latency_ms") or 0) for row in results) / max(1, len(results)), 2),
        "failover_targets": failover_targets,
        "failover_success": failover_success,
        "failover_failures": len(failover_results) - failover_success,
        "ok": success == len(results) and failover_success == len(failover_results),
    }
    return payload


def main() -> None:
    payload = asyncio.run(run())
    REPORT_PATH.parent.mkdir(parents=True, exist_ok=True)
    REPORT_PATH.write_text(json.dumps(payload, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")
    print(str(REPORT_PATH))


if __name__ == "__main__":
    main()

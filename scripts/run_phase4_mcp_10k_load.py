from __future__ import annotations

import asyncio
import json
import time
from datetime import datetime, timezone
from pathlib import Path

from app.blueprint.mcp.contracts import MCPRunContext
from app.blueprint.mcp.hub import get_mcp_client_hub
from app.blueprint.mcp.mock_servers import mock_tools_for
from app.blueprint.mcp.wave1_catalog import bootstrap_wave1_servers
from app.db.database import SessionLocal

REPORT_PATH = Path("docs/reports/phase4_mcp_10k_load.json")


async def _call(server_id: str, tool_name: str, *, user_id: str, idx: int, semaphore: asyncio.Semaphore) -> dict[str, object]:
    async with semaphore:
        db = SessionLocal()
        try:
            hub = get_mcp_client_hub()
            started = time.perf_counter()
            result = await hub.call_tool(
                db,
                server_id=server_id,
                tool_name=tool_name,
                arguments={"i": idx},
                run_context=MCPRunContext(
                    run_id="phase4-10k-load",
                    user_id=user_id,
                    provenance="user_direct",
                ),
            )
            return {
                "ok": not bool(result.is_error),
                "latency_ms": int((time.perf_counter() - started) * 1000),
            }
        except Exception as exc:  # pragma: no cover - script mode
            return {"ok": False, "error": str(exc), "latency_ms": 0}
        finally:
            db.close()


async def run(total_calls: int = 10_000, max_in_flight: int = 500) -> dict[str, object]:
    user_id = "phase4-10k-load-user"
    db = SessionLocal()
    try:
        summary = await bootstrap_wave1_servers(db, user_id=user_id, transport_mode="mock", connect=True)
    finally:
        db.close()

    servers = [str(item.get("server_id") or "") for item in (summary.get("items") or []) if str(item.get("server_id") or "")]
    if not servers:
        raise RuntimeError("No servers available for 10k load test")
    tool_map: dict[str, str] = {}
    for server_id in servers:
        tools = mock_tools_for(server_id)
        tool_map[server_id] = str((tools[0].name if tools else "health.check") or "health.check")

    semaphore = asyncio.Semaphore(max(10, int(max_in_flight)))
    tasks = [
        _call(
            servers[i % len(servers)],
            tool_map.get(servers[i % len(servers)]) or "health.check",
            user_id=f"{user_id}-{i}",
            idx=i,
            semaphore=semaphore,
        )
        for i in range(int(total_calls))
    ]

    started = time.perf_counter()
    results = await asyncio.gather(*tasks)
    elapsed_s = time.perf_counter() - started

    ok_count = sum(1 for row in results if bool(row.get("ok")))
    fail_count = len(results) - ok_count
    latencies = [int(row.get("latency_ms") or 0) for row in results]
    latencies.sort()

    p95 = latencies[int(0.95 * (len(latencies) - 1))] if latencies else 0
    return {
        "generated_at": datetime.now(timezone.utc).isoformat(),
        "total_calls": len(results),
        "max_in_flight": max_in_flight,
        "ok": fail_count == 0,
        "success_count": ok_count,
        "failure_count": fail_count,
        "elapsed_seconds": round(elapsed_s, 3),
        "throughput_rps": round(len(results) / max(0.001, elapsed_s), 2),
        "latency_p95_ms": int(p95),
    }


def main() -> None:
    payload = asyncio.run(run())
    REPORT_PATH.parent.mkdir(parents=True, exist_ok=True)
    REPORT_PATH.write_text(json.dumps(payload, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")
    print(str(REPORT_PATH))


if __name__ == "__main__":
    main()

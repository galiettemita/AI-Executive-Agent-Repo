from __future__ import annotations

import hashlib
import json
import time

from fastapi import APIRouter, HTTPException

from app.blueprint.contracts import ToolCall, ToolResult
from app.blueprint.db import get_tool_execution_by_idempotency, insert_tool_execution
from app.db.database import SessionLocal
from app.services.tavily_client import TavilyNotConfiguredError, tavily_search


router = APIRouter(prefix="/internal/hands", tags=["internal-hands"])


def _default_idempotency_key(*, tool: str, args: dict) -> str:
    payload = json.dumps({"tool": tool, "args": args or {}}, sort_keys=True, ensure_ascii=False).encode("utf-8")
    return f"hands:{tool}:{hashlib.sha256(payload).hexdigest()}"


@router.post("/execute", response_model=ToolResult)
async def execute(call: ToolCall) -> ToolResult:
    """
    Hands Plane: tool execution endpoint (Phase 1).

    Supported tools:
    - web.search (Tavily)
    """
    tool = (call.tool or "").strip()
    args = call.args or {}
    idempotency_key = (call.idempotency_key or "").strip() or _default_idempotency_key(tool=tool, args=args)

    # Idempotency: if already executed, return the stored output.
    if call.user_id:
        db = SessionLocal()
        try:
            existing = get_tool_execution_by_idempotency(db, user_id=call.user_id, idempotency_key=idempotency_key)
        except Exception:
            existing = None
        finally:
            try:
                db.close()
            except Exception:
                pass
        if existing and existing.get("output") is not None:
            status = str(existing.get("status") or "")
            ok = status == "success"
            return ToolResult(
                tool=tool,
                ok=ok,
                result=existing.get("output") if ok else None,
                error=None if ok else json.dumps(existing.get("error") or {}, ensure_ascii=False),
            )

    if tool in ("web.search", "tavily.search"):
        query = str(args.get("query") or "").strip()
        if not query:
            raise HTTPException(status_code=400, detail="Missing args.query")

        started = time.perf_counter()
        try:
            data = await tavily_search(query, max_results=5)
        except TavilyNotConfiguredError as exc:
            result = ToolResult(tool=tool, ok=False, error=str(exc))
            status = "failed"
            output_payload = None
            error_payload = {"type": "not_configured", "message": str(exc)}
        except Exception as exc:
            result = ToolResult(tool=tool, ok=False, error=str(exc))
            status = "failed"
            output_payload = None
            error_payload = {"type": exc.__class__.__name__, "message": str(exc)}
        else:
            output_payload = {"query": query, "data": data}
            result = ToolResult(tool=tool, ok=True, result=output_payload)
            status = "success"
            error_payload = None

        latency_ms = int((time.perf_counter() - started) * 1000)

        # Best-effort logging to blueprint table.
        if call.user_id and call.run_id:
            db = SessionLocal()
            try:
                insert_tool_execution(
                    db,
                    run_id=call.run_id,
                    user_id=call.user_id,
                    tool_name=tool,
                    input_payload={"args": args},
                    output_payload=output_payload,
                    status=status,
                    error_payload=error_payload,
                    idempotency_key=idempotency_key,
                    risk_level="none",
                    cost_cents=0,
                    latency_ms=latency_ms,
                )
            except Exception:
                pass
            finally:
                try:
                    db.close()
                except Exception:
                    pass

        return result

    return ToolResult(tool=tool, ok=False, error=f"Unknown tool: {tool}")


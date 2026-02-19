from __future__ import annotations

from typing import Any

from sqlalchemy.orm import Session

from app.blueprint.mcp.hub import get_mcp_client_hub
from app.blueprint.mcp.registry import MCPServerRegistry


async def activate_server(
    db: Session,
    *,
    user_id: str,
    server_id: str,
) -> dict[str, Any]:
    registry = MCPServerRegistry()
    registry.ensure_tables(db)

    try:
        _ = registry.get_server_config(db, server_id)
    except Exception as exc:
        return {"ok": False, "server_id": server_id, "error": f"server_not_registered: {exc}"}

    hub = get_mcp_client_hub()
    await hub.initialize(db)
    try:
        result = await hub.connect_server(db, user_id=user_id, server_id=server_id)
        return {
            "ok": bool(result.get("connected")),
            "server_id": server_id,
            "connected": bool(result.get("connected")),
            "result": result,
        }
    except Exception as exc:
        return {"ok": False, "server_id": server_id, "error": str(exc)}

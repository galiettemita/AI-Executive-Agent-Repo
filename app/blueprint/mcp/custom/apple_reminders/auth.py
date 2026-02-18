from __future__ import annotations

import os


def is_request_authorized(token: str | None) -> bool:
    expected = str(os.getenv("APPLE_REMINDERS_MCP_TOKEN") or "").strip()
    if not expected:
        return True
    return str(token or "").strip() == expected


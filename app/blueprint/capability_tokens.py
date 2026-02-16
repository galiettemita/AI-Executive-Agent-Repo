from __future__ import annotations

import time
from dataclasses import dataclass
from typing import Any

import jwt

from app.blueprint.contracts import ContentProvenance
from app.blueprint.security import CapabilityViolation, PRIVILEGE_LEVELS
from app.core.config import settings


@dataclass(frozen=True)
class CapabilityClaims:
    token_id: str
    run_id: str
    user_id: str
    issued_at: int
    expires_at: int
    provenance: str
    allowed_tools: list[str]
    capabilities: list[str]


def _jwt_secret() -> str:
    if not settings.JWT_SECRET:
        raise RuntimeError("JWT_SECRET is not configured")
    return settings.JWT_SECRET


def _policy_tools_for_provenance(provenance: ContentProvenance) -> list[str]:
    policy = PRIVILEGE_LEVELS.get(provenance, PRIVILEGE_LEVELS[ContentProvenance.USER_DIRECT])
    if policy.allowed_tools is None:
        return ["*"]
    return sorted(policy.allowed_tools)


def issue_capability_token(
    *,
    run_id: str,
    user_id: str,
    provenance: ContentProvenance,
    capabilities: list[str] | None = None,
    ttl_seconds: int = 30 * 60,
) -> str:
    now = int(time.time())
    payload: dict[str, Any] = {
        "token_id": f"cap-{run_id}",
        "run_id": run_id,
        "user_id": user_id,
        "iat": now,
        "exp": now + max(60, int(ttl_seconds)),
        "provenance": provenance.value,
        "allowed_tools": _policy_tools_for_provenance(provenance),
        "capabilities": sorted(set(capabilities or [])),
    }
    return jwt.encode(payload, _jwt_secret(), algorithm="HS256")


def decode_capability_token(token: str) -> CapabilityClaims:
    try:
        data = jwt.decode(token, _jwt_secret(), algorithms=["HS256"])
    except Exception as exc:
        raise CapabilityViolation(f"Invalid capability token: {exc}") from exc
    return CapabilityClaims(
        token_id=str(data.get("token_id") or ""),
        run_id=str(data.get("run_id") or ""),
        user_id=str(data.get("user_id") or ""),
        issued_at=int(data.get("iat") or 0),
        expires_at=int(data.get("exp") or 0),
        provenance=str(data.get("provenance") or ContentProvenance.USER_DIRECT.value),
        allowed_tools=[str(x) for x in (data.get("allowed_tools") or [])],
        capabilities=[str(x) for x in (data.get("capabilities") or [])],
    )


def enforce_capability_token(
    *,
    token: str | None,
    run_id: str | None,
    user_id: str | None,
    tool_name: str,
    required_capabilities: list[str] | None = None,
) -> list[str]:
    if not token:
        raise CapabilityViolation("Missing capability token")
    claims = decode_capability_token(token)

    if run_id and claims.run_id and str(run_id) != claims.run_id:
        raise CapabilityViolation("Capability token run mismatch")
    if user_id and claims.user_id and str(user_id) != claims.user_id:
        raise CapabilityViolation("Capability token user mismatch")

    allowed_tools = set(claims.allowed_tools or [])
    if "*" not in allowed_tools and tool_name not in allowed_tools:
        raise CapabilityViolation(f"Tool {tool_name} not permitted by capability token")

    required = set(required_capabilities or [])
    granted = set(claims.capabilities or [])
    missing = sorted(required - granted)
    if missing:
        raise CapabilityViolation(f"Capability token missing: {', '.join(missing)}")
    return sorted(granted)

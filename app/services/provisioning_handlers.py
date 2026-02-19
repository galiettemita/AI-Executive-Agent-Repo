from __future__ import annotations

from dataclasses import dataclass
from typing import Protocol

from app.core.config import settings
from app.services.provisioning_sessions import create_provisioning_session
from app.services.url_shortener import shorten_url


@dataclass(frozen=True)
class ProvisionAuthContext:
    request_id: str
    user_id: str
    server_id: str
    reason: str
    original_task_id: str | None = None


class AuthHandler(Protocol):
    def begin(self, context: ProvisionAuthContext) -> dict[str, object]:
        ...


class OAuthProvisionHandler:
    def begin(self, context: ProvisionAuthContext) -> dict[str, object]:
        session_token = create_provisioning_session(
            {
                "request_id": context.request_id,
                "user_id": context.user_id,
                "server_id": context.server_id,
                "original_task_id": context.original_task_id,
            },
            ttl_seconds=15 * 60,
        )
        base = (settings.APP_BASE_URL or "").rstrip("/")
        callback_url = f"{base}/api/v1/provision/callback"
        oauth_url = f"{callback_url}?state={session_token}&code=pending"
        short = shorten_url(oauth_url, ttl_seconds=15 * 60)
        return {
            "status": "awaiting_auth",
            "auth_type": "oauth2",
            "auth_url": oauth_url,
            "short_auth_url": short.get("short_url"),
            "session_token": session_token,
            "message": "Tap the secure link to authorize this server, then return to chat.",
        }


class ApiKeyProvisionHandler:
    def begin(self, context: ProvisionAuthContext) -> dict[str, object]:
        return {
            "status": "awaiting_auth",
            "auth_type": "api_key",
            "message": (
                f"To connect {context.server_id}, send your API key in the secure connect flow. "
                "This environment currently stores only encrypted placeholders."
            ),
        }


class PreProvisionedHandler:
    def begin(self, context: ProvisionAuthContext) -> dict[str, object]:
        return {
            "status": "auth_received",
            "auth_type": "pre_provisioned",
            "message": f"{context.server_id} does not require user authorization.",
        }


class OAuthConsolidatedHandler(OAuthProvisionHandler):
    def begin(self, context: ProvisionAuthContext) -> dict[str, object]:
        payload = super().begin(context)
        payload["auth_type"] = "oauth2_consolidated"
        payload["message"] = "Use this single link to connect the full suite for this provider."
        return payload


class PlaidLinkProvisionHandler(OAuthProvisionHandler):
    def begin(self, context: ProvisionAuthContext) -> dict[str, object]:
        payload = super().begin(context)
        payload["auth_type"] = "plaid_link"
        payload["message"] = "Open Plaid Link and complete bank authorization, then return to chat."
        return payload


class TeslaSSOProvisionHandler(OAuthProvisionHandler):
    def begin(self, context: ProvisionAuthContext) -> dict[str, object]:
        payload = super().begin(context)
        payload["auth_type"] = "tesla_sso"
        payload["message"] = "Authorize Tesla SSO and return to chat to finish setup."
        return payload


def get_auth_handler(auth_type: str) -> AuthHandler:
    key = str(auth_type or "oauth2").strip().lower()
    if key in {"oauth2_consolidated"}:
        return OAuthConsolidatedHandler()
    if key in {"plaid_link"}:
        return PlaidLinkProvisionHandler()
    if key in {"tesla_sso"}:
        return TeslaSSOProvisionHandler()
    if key in {"api_key", "apikey"}:
        return ApiKeyProvisionHandler()
    if key in {"pre_provisioned", "none", "internal"}:
        return PreProvisionedHandler()
    # oauth2, oauth2_consolidated, plaid_link, tesla_sso follow link-based auth flow.
    return OAuthProvisionHandler()

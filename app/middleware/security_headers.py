from __future__ import annotations

from starlette.middleware.base import BaseHTTPMiddleware
from starlette.requests import Request

from app.core.config import settings


class SecurityHeadersMiddleware(BaseHTTPMiddleware):
    """
    Injects security headers for all responses.

    - HSTS in staging/production (configurable)
    - Basic hardening headers (X-Content-Type-Options, X-Frame-Options, etc.)
    - CSP on HTML responses only
    """

    async def dispatch(self, request: Request, call_next):
        response = await call_next(request)

        if settings.SECURITY_HEADERS_ENABLED != "1":
            return response

        response.headers.setdefault("X-Content-Type-Options", "nosniff")
        response.headers.setdefault("X-Frame-Options", "DENY")
        response.headers.setdefault("Referrer-Policy", "strict-origin-when-cross-origin")
        response.headers.setdefault("Permissions-Policy", "geolocation=(self)")
        response.headers.setdefault("Cross-Origin-Resource-Policy", "same-site")

        if settings.SECURITY_HSTS_ENABLED == "1" and settings.ENV in ("production", "staging"):
            max_age = settings.SECURITY_HSTS_MAX_AGE
            response.headers.setdefault(
                "Strict-Transport-Security",
                f"max-age={max_age}; includeSubDomains; preload",
            )

        content_type = response.headers.get("content-type", "")
        if "text/html" in content_type:
            csp_value = settings.SECURITY_CSP
            if settings.SECURITY_CSP_REPORT_ONLY == "1":
                response.headers.setdefault("Content-Security-Policy-Report-Only", csp_value)
            else:
                response.headers.setdefault("Content-Security-Policy", csp_value)

        return response

# app/middleware/rate_limiter.py
"""
Rate Limiting Middleware

Implements per-user and per-IP rate limiting to prevent abuse.

Default limits:
- 10 requests per minute per authenticated user
- 100 requests per minute per IP address

Uses in-memory storage by default, with optional Redis backend for production.
"""

from __future__ import annotations

import os
from typing import Optional, Callable

from fastapi import Request, Response
from slowapi import Limiter, _rate_limit_exceeded_handler
from slowapi.errors import RateLimitExceeded
from slowapi.util import get_remote_address


def get_user_identifier(request: Request) -> str:
    """
    Extract user identifier for rate limiting.

    Priority:
    1. Authenticated user ID from request state
    2. User ID from authorization header (JWT)
    3. X-User-ID header (for internal calls)
    4. IP address as fallback
    """
    # Check if user_id was set by auth middleware
    if hasattr(request.state, "user_id") and request.state.user_id:
        return f"user:{request.state.user_id}"

    # Check X-User-ID header (used by WhatsApp webhook and internal services)
    user_id_header = request.headers.get("X-User-ID")
    if user_id_header:
        return f"user:{user_id_header}"

    # Check query parameter (used in some endpoints)
    user_id_param = request.query_params.get("user_id")
    if user_id_param:
        return f"user:{user_id_param}"

    # Fall back to IP address
    return f"ip:{get_remote_address(request)}"


def get_ip_identifier(request: Request) -> str:
    """Get IP address for rate limiting."""
    return f"ip:{get_remote_address(request)}"


def create_limiter() -> Limiter:
    """
    Create and configure the rate limiter.

    Uses Redis if REDIS_URL is set, otherwise uses in-memory storage.
    """
    redis_url = os.getenv("REDIS_URL")

    if redis_url:
        # Use Redis for distributed rate limiting in production
        from slowapi.middleware import SlowAPIMiddleware
        return Limiter(
            key_func=get_user_identifier,
            storage_uri=redis_url,
            strategy="fixed-window",
        )
    else:
        # Use in-memory storage for development/single-instance
        return Limiter(
            key_func=get_user_identifier,
            strategy="fixed-window",
        )


# Create global limiter instance
limiter = create_limiter()


# Rate limit configurations
USER_RATE_LIMIT = os.getenv("RATE_LIMIT_USER", "10/minute")
IP_RATE_LIMIT = os.getenv("RATE_LIMIT_IP", "100/minute")

# Stricter limits for sensitive endpoints
AUTH_RATE_LIMIT = "5/minute"  # Login/registration attempts
PAYMENT_RATE_LIMIT = "20/minute"  # Payment operations
WEBHOOK_RATE_LIMIT = "1000/minute"  # Webhooks need higher limits


def rate_limit_user(limit: str = USER_RATE_LIMIT) -> Callable:
    """
    Decorator for user-based rate limiting.

    Usage:
        @router.get("/endpoint")
        @rate_limit_user()
        async def endpoint(request: Request):
            ...
    """
    return limiter.limit(limit, key_func=get_user_identifier)


def rate_limit_ip(limit: str = IP_RATE_LIMIT) -> Callable:
    """
    Decorator for IP-based rate limiting.

    Usage:
        @router.get("/endpoint")
        @rate_limit_ip()
        async def endpoint(request: Request):
            ...
    """
    return limiter.limit(limit, key_func=get_ip_identifier)


def rate_limit_auth() -> Callable:
    """Rate limit for authentication endpoints."""
    return limiter.limit(AUTH_RATE_LIMIT, key_func=get_ip_identifier)


def rate_limit_payment() -> Callable:
    """Rate limit for payment endpoints."""
    return limiter.limit(PAYMENT_RATE_LIMIT, key_func=get_user_identifier)


def rate_limit_webhook() -> Callable:
    """Rate limit for webhook endpoints (higher limit)."""
    return limiter.limit(WEBHOOK_RATE_LIMIT, key_func=get_ip_identifier)


# Exempt paths from rate limiting
EXEMPT_PATHS = {
    "/health",
    "/",
    "/docs",
    "/openapi.json",
    "/redoc",
}


def should_exempt(request: Request) -> bool:
    """Check if request path should be exempt from rate limiting."""
    return request.url.path in EXEMPT_PATHS


def setup_rate_limiting(app):
    """
    Setup rate limiting middleware on FastAPI app.

    Args:
        app: FastAPI application instance
    """
    from slowapi.middleware import SlowAPIMiddleware
    from slowapi.errors import RateLimitExceeded

    # Add limiter to app state
    app.state.limiter = limiter

    # Add middleware
    app.add_middleware(SlowAPIMiddleware)

    # Add exception handler
    app.add_exception_handler(RateLimitExceeded, _rate_limit_exceeded_handler)

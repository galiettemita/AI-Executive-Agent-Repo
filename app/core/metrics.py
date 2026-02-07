from __future__ import annotations

import time
from typing import Callable

try:
    import prometheus_client as prom
except ModuleNotFoundError:  # pragma: no cover - only in minimal environments
    prom = None
from starlette.middleware.base import BaseHTTPMiddleware
from starlette.requests import Request
from starlette.responses import Response


REQUEST_COUNT = None
REQUEST_LATENCY = None


def _ensure_metrics():
    global REQUEST_COUNT, REQUEST_LATENCY
    if prom is None:
        raise RuntimeError("prometheus_client is not installed")
    if REQUEST_COUNT is None:
        REQUEST_COUNT = prom.Counter(
            "http_requests_total",
            "Total HTTP requests",
            ["method", "path", "status"],
        )
    if REQUEST_LATENCY is None:
        REQUEST_LATENCY = prom.Histogram(
            "http_request_duration_seconds",
            "HTTP request duration in seconds",
            ["method", "path"],
        )


def _route_path(request: Request) -> str:
    route = request.scope.get("route")
    if route and hasattr(route, "path"):
        return route.path
    return request.url.path


class MetricsMiddleware(BaseHTTPMiddleware):
    async def dispatch(self, request: Request, call_next: Callable):
        _ensure_metrics()
        start = time.perf_counter()
        response = await call_next(request)
        duration = time.perf_counter() - start

        path = _route_path(request)
        REQUEST_COUNT.labels(request.method, path, response.status_code).inc()
        REQUEST_LATENCY.labels(request.method, path).observe(duration)

        return response


def metrics_response() -> Response:
    _ensure_metrics()
    payload = prom.generate_latest()
    return Response(content=payload, media_type=prom.CONTENT_TYPE_LATEST)

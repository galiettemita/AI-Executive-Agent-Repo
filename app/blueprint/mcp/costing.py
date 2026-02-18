from __future__ import annotations

import time
from collections import defaultdict, deque
from dataclasses import dataclass
from threading import Lock

from app.core.redis import get_redis


@dataclass
class CostRecord:
    cost_cents: float
    latency_ms: int


class MCPCostTracker:
    """
    Tracks per-call cost, per-run cost, and per-server daily budgets.
    Falls back to in-memory counters when Redis is unavailable.
    """

    def __init__(self) -> None:
        self._lock = Lock()
        self._daily_server_cost: dict[tuple[str, str], float] = defaultdict(float)
        self._run_cost: dict[str, float] = defaultdict(float)

    def _day_key(self) -> str:
        return time.strftime("%Y-%m-%d", time.gmtime())

    def _redis_key(self, server_id: str, user_id: str | None) -> str:
        suffix = user_id or "anonymous"
        return f"mcp:cost:{self._day_key()}:{server_id}:{suffix}"

    def record(
        self,
        *,
        server_id: str,
        user_id: str | None,
        run_id: str | None,
        latency_ms: int,
    ) -> float:
        # heuristic latency-based proxy until provider-native cost metadata is available
        base = 0.05
        latency_component = max(0.0, float(latency_ms) / 1000.0) * 0.2
        cost_cents = round(base + latency_component, 4)

        redis_client = get_redis()
        if redis_client:
            key = self._redis_key(server_id, user_id)
            try:
                redis_client.incrbyfloat(key, cost_cents)
                redis_client.expire(key, 60 * 60 * 26)
            except Exception:
                pass

        with self._lock:
            daily_key = (self._day_key(), server_id)
            self._daily_server_cost[daily_key] += cost_cents
            if run_id:
                self._run_cost[run_id] += cost_cents

        return cost_cents

    def get_daily_server_cost(self, server_id: str) -> float:
        with self._lock:
            return float(self._daily_server_cost.get((self._day_key(), server_id), 0.0))

    def get_run_cost(self, run_id: str | None) -> float:
        if not run_id:
            return 0.0
        with self._lock:
            return float(self._run_cost.get(run_id, 0.0))


class MCPRateLimiter:
    def __init__(self) -> None:
        self._lock = Lock()
        self._calls: dict[tuple[str, str], deque[float]] = defaultdict(deque)

    def allow(self, *, server_id: str, user_id: str | None, per_min: int) -> bool:
        limit = max(1, int(per_min))
        now = time.monotonic()
        key = (server_id, user_id or "anonymous")

        with self._lock:
            q = self._calls[key]
            while q and (now - q[0]) > 60:
                q.popleft()
            if len(q) >= limit:
                return False
            q.append(now)
            return True

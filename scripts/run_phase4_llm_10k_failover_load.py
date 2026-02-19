from __future__ import annotations

import concurrent.futures
import json
import logging
import threading
from datetime import datetime, timezone
from pathlib import Path

from app.blueprint.contracts import LLMProvider, LLMProviderHealth, LLMRequest, LLMResponse, TokenUsage
from app.blueprint.llm.router import LLMRouter

REPORT_PATH = Path("docs/reports/phase4_llm_10k_failover_load.json")
logging.getLogger("app.blueprint.llm.router").setLevel(logging.ERROR)


def _health_map() -> dict[str, LLMProviderHealth]:
    return {
        LLMProvider.OPENAI.value: LLMProviderHealth(
            provider=LLMProvider.OPENAI,
            is_healthy=True,
            latency_p95_ms=180,
            error_rate_1h=0.0,
        ),
        LLMProvider.ANTHROPIC.value: LLMProviderHealth(
            provider=LLMProvider.ANTHROPIC,
            is_healthy=True,
            latency_p95_ms=260,
            error_rate_1h=0.0,
        ),
        LLMProvider.GOOGLE.value: LLMProviderHealth(
            provider=LLMProvider.GOOGLE,
            is_healthy=False,
            latency_p95_ms=400,
            error_rate_1h=1.0,
        ),
        LLMProvider.LOCAL.value: LLMProviderHealth(
            provider=LLMProvider.LOCAL,
            is_healthy=True,
            latency_p95_ms=120,
            error_rate_1h=0.0,
        ),
    }


def run(*, total_requests: int = 10_000, max_workers: int = 200) -> dict[str, object]:
    router = LLMRouter()
    health_map = _health_map()

    router.all_provider_health = lambda force_refresh=False: health_map
    router.provider_health = lambda provider, force_refresh=False: health_map.get(provider.value) or LLMProviderHealth(
        provider=provider,
        is_healthy=False,
        error_rate_1h=1.0,
    )
    router._provider_enabled = lambda provider: provider in {
        LLMProvider.OPENAI,
        LLMProvider.ANTHROPIC,
        LLMProvider.LOCAL,
    }

    lock = threading.Lock()
    call_counter = {"n": 0}

    def _fake_call_provider(provider: LLMProvider, model: str, req: LLMRequest):
        with lock:
            call_counter["n"] += 1
            n = int(call_counter["n"])
        if provider == LLMProvider.OPENAI and n % 12 == 0:
            raise RuntimeError("simulated_openai_failure")

        latency_ms = 140
        if provider == LLMProvider.ANTHROPIC:
            latency_ms = 220
        elif provider == LLMProvider.LOCAL:
            latency_ms = 110

        usage = TokenUsage(input_tokens=220, output_tokens=180, total_tokens=400)
        cost_cents = router._estimate_cost_cents(model=model, usage=usage)
        return (
            LLMResponse(
                provider=provider,
                model=model,
                content="ok",
                usage=usage,
                cost_cents=cost_cents,
                latency_ms=latency_ms,
            ),
            None,
        )

    router._call_provider = _fake_call_provider

    def _single(idx: int) -> dict[str, object]:
        req = LLMRequest(
            user_id=f"phase4-llm-load-user-{idx}",
            messages=[{"role": "user", "content": f"Summarize task {idx}"}],
            task_type="general",
            max_latency_ms=3000,
            max_cost_cents=5.0,
        )
        try:
            result = router.call(req)
            return {
                "ok": True,
                "provider": result.provider.value,
                "latency_ms": int(result.latency_ms or 0),
                "cost_cents": float(result.cost_cents or 0.0),
                "was_failover": bool(result.was_failover),
            }
        except Exception as exc:  # pragma: no cover - script path
            return {"ok": False, "error": str(exc)}

    with concurrent.futures.ThreadPoolExecutor(max_workers=max_workers) as pool:
        futures = [pool.submit(_single, idx) for idx in range(int(total_requests))]
        rows = [f.result() for f in concurrent.futures.as_completed(futures)]

    successes = [row for row in rows if bool(row.get("ok"))]
    failures = [row for row in rows if not bool(row.get("ok"))]
    latencies = sorted(int(row.get("latency_ms") or 0) for row in successes)
    p95 = latencies[int(0.95 * (len(latencies) - 1))] if latencies else 0

    avg_cost_cents = (sum(float(row.get("cost_cents") or 0.0) for row in successes) / len(successes)) if successes else 0.0
    blended_cost_dollars = round(avg_cost_cents / 100.0, 6)

    failover_count = sum(1 for row in successes if bool(row.get("was_failover")))
    provider_breakdown: dict[str, int] = {}
    for row in successes:
        provider = str(row.get("provider") or "unknown")
        provider_breakdown[provider] = int(provider_breakdown.get(provider) or 0) + 1

    latency_target_ms = 3000
    cost_target_dollars = 0.035

    payload = {
        "generated_at": datetime.now(timezone.utc).isoformat(),
        "total_requests": int(total_requests),
        "max_workers": int(max_workers),
        "success_count": len(successes),
        "failure_count": len(failures),
        "p95_latency_ms": int(p95),
        "latency_target_ms": latency_target_ms,
        "avg_cost_cents": round(avg_cost_cents, 4),
        "blended_cost_dollars": blended_cost_dollars,
        "cost_target_dollars": cost_target_dollars,
        "failover_count": int(failover_count),
        "provider_breakdown": provider_breakdown,
        "latency_target_met": p95 < latency_target_ms,
        "cost_target_met": blended_cost_dollars < cost_target_dollars,
        "failover_observed": failover_count > 0,
    }
    payload["ok"] = (
        payload["failure_count"] == 0
        and bool(payload["latency_target_met"])
        and bool(payload["cost_target_met"])
        and bool(payload["failover_observed"])
    )
    return payload


def main() -> None:
    payload = run()
    REPORT_PATH.parent.mkdir(parents=True, exist_ok=True)
    REPORT_PATH.write_text(json.dumps(payload, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")
    print(str(REPORT_PATH))


if __name__ == "__main__":
    main()

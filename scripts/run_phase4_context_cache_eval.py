from __future__ import annotations

import json
from datetime import datetime, timezone
from pathlib import Path

from app.blueprint.context_compiler import (
    compile_context_messages,
    context_cache_stats,
    precompute_context_blocks,
    reset_context_cache_stats,
)

REPORT_PATH = Path("docs/reports/phase4_context_cache_eval.json")


def run(*, iterations: int = 120) -> dict[str, object]:
    user_id = "phase4-context-cache-user"
    reset_context_cache_stats()

    _ = compile_context_messages(
        base_system_prompt="You are Executive OS.",
        user_id=user_id,
        user_text="warmup",
        history_messages=[],
        tier=1,
    )

    precompute = precompute_context_blocks(
        user_id=user_id,
        base_system_prompt="You are Executive OS.",
        tiers=(1, 2, 3),
    )

    for idx in range(max(1, int(iterations))):
        _ = compile_context_messages(
            base_system_prompt="You are Executive OS.",
            user_id=user_id,
            user_text=f"iteration-{idx}",
            history_messages=[],
            tier=1,
        )

    stats = context_cache_stats()
    hits = int(stats.get("hits") or 0)
    misses = int(stats.get("misses") or 0)
    total = hits + misses
    hit_rate = round((hits / total) * 100.0, 2) if total > 0 else 0.0

    return {
        "generated_at": datetime.now(timezone.utc).isoformat(),
        "precompute": precompute,
        "iterations": int(iterations),
        "hits": hits,
        "misses": misses,
        "hit_rate_percent": hit_rate,
        "target_percent": 20.0,
        "ok": hit_rate >= 20.0,
    }


def main() -> None:
    payload = run()
    REPORT_PATH.parent.mkdir(parents=True, exist_ok=True)
    REPORT_PATH.write_text(json.dumps(payload, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")
    print(str(REPORT_PATH))


if __name__ == "__main__":
    main()

from __future__ import annotations

import re


_T0_PATTERNS = [
    re.compile(r"^(hi|hello|hey|yo|good (morning|afternoon|evening))\\b", re.I),
    re.compile(r"^(thanks|thank you|thx)\\b", re.I),
    re.compile(r"^(ok|okay|k)\\b", re.I),
]


def route_tier(user_text: str) -> int:
    """
    Blueprint tier routing (Phase 1):
    - T0: instant for greetings/thanks/acks
    - T1: default for everything else

    We’ll expand to T2/T3 as we implement ReAct/Temporal.
    """
    text = (user_text or "").strip()
    if not text:
        return 0
    for pat in _T0_PATTERNS:
        if pat.search(text):
            return 0
    return 1


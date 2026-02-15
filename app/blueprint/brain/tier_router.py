from __future__ import annotations

import re


_T0_PATTERNS = [
    re.compile(r"^(hi|hello|hey|yo|good (morning|afternoon|evening))\\b", re.I),
    re.compile(r"^(thanks|thank you|thx)\\b", re.I),
    re.compile(r"^(ok|okay|k)\\b", re.I),
]

_T3_HINTS = [
    re.compile(r"\\b(multi-step|step by step|roadmap|strategy|analyze deeply|think deeply)\\b", re.I),
]

_T2_HINTS = [
    re.compile(r"\\b(search|find|look up|research)\\b", re.I),
    re.compile(r"\\b(price|buy|purchase|shop)\\b", re.I),
    re.compile(r"\\b(book|reserve|flight|hotel|itinerary)\\b", re.I),
]


def route_tier(user_text: str) -> int:
    """
    Blueprint tier routing (Phase 1):
    - T0: instant for greetings/thanks/acks
    - T1: default for everything else
    - T2: tool-assisted (web search, bookings) heuristics
    - T3: complex planning heuristics

    We’ll expand to T2/T3 as we implement ReAct/Temporal.
    """
    text = (user_text or "").strip()
    if not text:
        return 0
    for pat in _T0_PATTERNS:
        if pat.search(text):
            return 0

    for pat in _T3_HINTS:
        if pat.search(text):
            return 3

    for pat in _T2_HINTS:
        if pat.search(text):
            return 2
    return 1

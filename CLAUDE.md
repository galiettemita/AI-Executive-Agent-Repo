Read over FOMO_DESIGN.md and SALVAGE_MAP.md. Those are your two lifelines.
Also read FOMO_PLAN.md — the strengthened implementation plan with safety gates, idempotency, the reply-parser pipeline, and the day-by-day breakdown.
Also read docs/future-architecture-notes.md — the long-term-assistant institutional memory. Every file archived during cleanup has a section there with the concept, critique, future-implementation guidance, and recovery pointer. Read it before designing any layer above L1 (Calendar / Drafting / Sending / MCP tools / Autonomous / Memory).
FRIENDS.md and OUTREACH.md are filling-in-the-blanks templates for the human-conversation tasks.

Do not treat Brevio's false-positive problem as a prompt-only bug. Personalized Importance Learning is a permanent product principle. Before changing ranker behavior, commercial/spam handling, the reply parser, feedback events, or memory signals, read `docs/personalized-importance-learning.md`. Brevio must learn each user's definition of important while avoiding cross-user leakage and over-aggressive suppression.

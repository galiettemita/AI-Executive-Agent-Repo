# Brevio Active Phase Contract

Harness anchor loaded: BREVIO-HARNESS-V1-NO-CIRCLING-FAST-SHIPPING

## Current phase

M1-B — no-migration typed facade over existing `memory_signals`.

## Approved decision

B only. No A/B/C relitigation. Existing `memory_signals` are the foundation, not completed M1 by themselves. Migration-bearing M1 is not approved without a fresh founder gate.

## Allowed without fresh founder approval

- typed read/query helpers
- validation hardening
- retrieval-pack helpers over existing `memory_signals`
- cross-user isolation tests
- deleted/tombstoned exclusion tests
- metadata preservation tests
- no-migration boundary tests
- dormant seams that do not activate runtime behavior
- docs reconciliation only when it directly unlocks implementation
- PR creation, CI wait, merge after green checks, and local sync for in-scope work

## Forbidden without fresh founder approval

- DB migration
- new table
- runtime consumer activation
- live ranking behavior change
- HMR behavior change
- reply-parser behavior change
- Calendar live activation
- Composio runtime
- Tool Gateway
- browser automation
- action tools
- production deploy
- irreversible data changes
- new OAuth/security scope
- major architecture fork

## Exit condition

M1-B is done only when:

- the M1-B PR queue is merged
- CI is green on the exact PR/commit being reported
- no forbidden surfaces were touched
- the typed facade over existing `memory_signals` has query/read helpers, validation hardening, retrieval-pack preparation, audit/evidence helpers where safe, and closeout proof
- a final M1-B report explains what is done, what remains deferred, and what next phase is unlocked

## Loop prevention

No cycle may report vague work such as “continue M1.” The cycle must name the current `NEXT` queue item from `.hermes/NEXT_PR_QUEUE.md` and either ship it, open a PR for it, produce concrete changed files for it, or name the exact blocker and owner/action.

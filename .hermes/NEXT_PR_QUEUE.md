# Brevio Next PR Queue

Harness anchor loaded: BREVIO-HARNESS-V1-NO-CIRCLING-FAST-SHIPPING

There must be exactly one item marked `NEXT`. No cycle may say “continue M1” or “continue Memory V1” vaguely. Move the marker only after the current item is merged or explicitly blocked with owner/action.

## NEXT — PR-G: Memory V1 why-used explanation

- **Exact branch:** `memory-v1-why-used-explanation`
- **Purpose:** Expose a user-trust explanation for visible memory recall: given a visible memory recall result, Brevio can answer “why did you use this memory?” using source/audit metadata without leaking private raw content or activating unsafe runtime behavior.
- **Memory V1 exit condition advanced:**
  3. explain why a memory was used;
  5. prove source/audit metadata;
  6. prevent cross-user leakage;
  7. expose at least one visible memory behavior to the user.
- **Allowed files/areas:**
  - `apps/fomo/src/memory/typed-memory-visible-recall.ts`
  - `apps/fomo/src/memory/typed-memory-visible-recall.test.ts`
  - at most one new small helper/test file under `apps/fomo/src/memory/` if the explanation surface is clearer as a separate helper
- **Forbidden files/areas:**
  - migrations
  - new tables or schema changes
  - production deploy
  - Calendar memory/live activation
  - Composio runtime memory
  - Tool Gateway
  - browser automation/action tools
  - ranker/HMR/reply-parser behavior changes
  - broad UI redesign or dashboard
  - large memory graph/advanced ranking/recency decay
  - new harness expansion
- **Expected changed files:** 1–3 memory files, no docs unless a test fixture/comment needs a one-line clarification.
- **Tests required:** targeted memory tests proving the why-used explanation is user-visible, source/audit-backed, cross-user isolated, and does not leak raw private content; plus full FOMO test/lint/build or CI for the exact PR commit.
- **Merge condition:** PR exists, CI green for the PR commit, diff stays inside allowed memory files, no forbidden surfaces touched, and tests demonstrate the “why did you use this memory?” explanation object/string.
- **Exit condition:** PR merged and local `main` synced; a test proves the explanation can be produced from visible recall metadata without broad runtime activation or private-content leakage.
- **Stop condition:** Stop and report owner/action if implementation requires migration/new table/runtime integration into live ranker/HMR/reply-parser/Calendar/Composio/Tool Gateway/browser/action tools/production deploy.
- **Founder approval needed?** No if helper/test-level or narrow visible local behavior only; yes if live production/user deployment or external action scope is touched.

## Completed

### PR-F: Memory V1 visible explicit-preference recall

- **Purpose:** Expose the first visible Memory V1 behavior: when Brevio has an explicit user preference in the M1-B substrate, a narrow tested helper can retrieve it and produce a safe user-visible explanation of why it was used.
- **Status:** Completed in PR #83, canonical commit `99c3d91c00504fd1a9e9a1d1fc186d4aabf4bb3c`, merged as `a6e768dab38997ae6fb617477b50fe63f90411a0`.
- **Done condition met:** PR merged and local `main` synced; tests proved explicit user preference recall returns safe user-visible explanation metadata with source/audit evidence and cross-user isolation.

### PR-E: M1-B closeout

- **Purpose:** Prove M1-B no-migration foundation is complete enough for Memory V1 and identify the Memory V1 startup-speed recommendation.
- **Status:** Completed in PR #82, merged as `b9652af1ea5aac875b427318f8fc25802a749873`.
- **Done condition met:** M1-B frozen; Memory V1 active phase started from concrete PR-F.

### PR-D: Memory audit/evidence helper surface, dormant only

- **Purpose:** Add evidence/debug helper that can explain which memory rows were considered/excluded.
- **Status:** Completed in PR #79, merged as `3778934c0e57f9f051c1cc3c61073aad79a19a5a`.

### PR-C: Typed retrieval pack builder, dormant only

- **Purpose:** Add retrieval-pack construction over existing `memory_signals` / typed facade without activating runtime consumers.
- **Status:** Completed in PR #78, merged as `c4ac65ca3dc2978837bdea547c0d6ac39ad0c9e9`.

### PR-B: M1 validation hardening

- **Purpose:** Add focused tests/validators for the current typed-memory facade and `memory_signals` bridge so M1-B safety is mechanically proven before retrieval-pack work.
- **Status:** Completed in PR #77, merged as `2f44f872e2022223afe5b0ac763657eff572d371`.

### PR-A: Publish and merge docs/rulebook reconcile

- **Purpose:** Get local docs/rulebook reconcile out of local limbo through branch → PR → CI → merge flow.
- **Status:** Completed in PR #75, merged as `de54b57fa8d08896a12eac53099e061666a4c02b`; local `main` synced to `origin/main`.

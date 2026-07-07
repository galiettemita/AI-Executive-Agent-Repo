# Brevio Next PR Queue

Harness anchor loaded: BREVIO-HARNESS-V1-NO-CIRCLING-FAST-SHIPPING

There must be exactly one item marked `NEXT`. No cycle may say “continue M1” or “continue Memory V1” vaguely. Move the marker only after the current item is merged or explicitly blocked with owner/action.

## NEXT — PR-J: Memory V1 closeout and next-phase gate

- **Exact branch:** `memory-v1-closeout-queue`
- **Purpose:** Close Memory V1 with evidence from PR-F through PR-I, update the active phase contract so the harness stops re-queuing completed Memory V1 work, and name the next smallest approved phase gate before any Feedback/Learn implementation starts.
- **Memory V1 exit condition advanced:** confirms all seven Memory V1 exit conditions are complete from merged code/tests, including remember → recall → explain → correct/forget for explicit preferences.
- **Brevio core dimensions advanced:** Memory Architecture, User Trust/Consent, Observability/Evals/Reliability. Preserves Security/Permission Gates and HMR. Defers Autonomy, Proactivity, Tool/Workflow Orchestration, Calendar, Composio, browser automation, and action tools.
- **Allowed files/areas:**
  - `.hermes/ACTIVE_PHASE_CONTRACT.md`
  - `.hermes/NEXT_PR_QUEUE.md`
  - optional one-line closeout note in an existing `.hermes/` closeout file if needed
- **Forbidden files/areas:**
  - product runtime code
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
- **Expected changed files:** 2 tracked harness/phase files, no product source changes.
- **Tests required:** `.hermes/verify-brevio-harness.sh`; `pnpm run lint`; `pnpm run test`; `pnpm run build` or clear evidence they already pass for the exact commit being closed out.
- **Merge condition:** PR exists, CI green for the PR commit, diff only updates tracked phase/queue state, Memory V1 closeout cites exact merged commits/tests, and the next implementation phase is a gate rather than silent runtime activation.
- **Exit condition:** PR merged and local `main` synced; exactly one NEXT item remains and it no longer points at completed PR-I.
- **Stop condition:** Stop and report owner/action if closeout requires production deploy, live behavior activation, migration, OAuth/security scope change, Calendar/Composio/Tool Gateway/browser/action tooling, or a strategic phase decision that Galiette has not approved.
- **Founder approval needed?** No for closeout/queue hygiene; yes before implementing a new strategic phase beyond the closeout gate.

## Completed

### PR-I: Memory V1 remember-this explicit preference

- **Purpose:** Add the next narrow visible Memory V1 trust behavior: prove a user-stated “remember this” explicit preference can be saved through a dormant/tested helper, then recalled, explained, corrected, and forgotten by the existing visible loop.
- **Status:** Completed on `main` at `1eb1f54ba5e1e00d2bdd4d0a95689ac1d3fc7f18`.
- **Done condition met:** Local `main` is synced with `origin/main`; tests prove remember → recall → explain → correct/forget for explicit user preferences, with source/audit metadata, cross-user isolation, private-value redaction, no migration/new table, and no runtime activation.

### PR-H: Memory V1 forget/correct explicit preference

- **Purpose:** Prove a user can forget or correct an explicit preference memory safely, with source/audit metadata preserved and no cross-user leakage.
- **Status:** Completed in PR #87, canonical commit `5f910b2581657d32a89ec622782d081cda2c4835`, merged as `d24b99ad79a993af0975183240a35e21a52a4731`.
- **Done condition met:** PR merged and local `main` synced; tests proved the visible loop: recall → why-used explanation → forget/correct → recall again, with old memory excluded and corrected memory used.

### PR-G: Memory V1 why-used explanation

- **Purpose:** Expose a user-trust explanation for visible memory recall: given a visible memory recall result, Brevio can answer “why did you use this memory?” using source/audit metadata without leaking private raw content or activating unsafe runtime behavior.
- **Status:** Completed in PR #85, canonical commit `b69718708557a282760126cd129b8a0720130e1a`, merged as `a558b7bc8c98a9bfd3f0667a9767b1021a45c243`.
- **Done condition met:** PR merged and local `main` synced; tests proved why-used explanation can be produced from visible recall metadata without broad runtime activation or private-content leakage.

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

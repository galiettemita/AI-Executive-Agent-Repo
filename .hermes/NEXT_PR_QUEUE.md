# Brevio Next PR Queue

Harness anchor loaded: BREVIO-HARNESS-V1-NO-CIRCLING-FAST-SHIPPING

There must be exactly one item marked `NEXT`. No cycle may say “continue M1” or “continue Memory V1” vaguely. Move the marker only after the current item is merged or explicitly blocked with owner/action.

## NEXT — PR-3: “Why did you remember/use that?” explanation

- **Exact branch:** `memory-v1-visible-why-remembered`
- **Purpose:** Expose a small explanation path so Brevio can explain memory use in a human-readable, privacy-safe way.
- **Memory V1 Visible Behavior exit condition advanced:**
  3. explain why a memory was used;
  5. prove source/audit metadata;
  6. prevent cross-user leakage;
  7. expose at least one visible memory behavior to the user.
- **Allowed files/areas:**
  - visible memory explanation helper or adapter for explicit preferences;
  - source/audit metadata formatting;
  - targeted tests proving human-readable explanation behavior;
  - narrow dormant helpers only when directly tied to this visible behavior.
- **Forbidden files/areas:**
  - product runtime activation beyond the narrow visible-memory explanation surface;
  - DB migrations;
  - new tables/schema changes;
  - production deploy;
  - Calendar live activation;
  - Composio runtime;
  - Tool Gateway;
  - browser automation/action tools;
  - broad HMR rewrite;
  - OAuth/security scope changes;
  - broad strategic phase fork.
- **Expected changed files:** a small implementation/test slice in the existing memory-visible behavior area; no broad docs/harness changes.
- **Tests required:** user-facing explanation is human-readable; no private audit leakage; source/audit metadata proof; cross-user isolation proof; deleted/tombstoned memories are not explained; CI for exact PR commit.
- **Merge condition:** PR exists, CI green for exact PR commit, diff stays inside approved Memory V1 visible behavior scope, no forbidden surfaces touched.
- **Exit condition:** PR merged and local `main` synced; explanation path proves why a visible memory was remembered/used without leaking private values.
- **Stop condition:** Stop and report owner/action if implementation requires migration/new table, production deploy, OAuth/security scope change, or activation of Calendar/Composio/Tool Gateway/browser/action tools.
- **Founder approval needed?** No for narrow helper/test-level explanation behavior; yes before production deploy, new external scopes, irreversible data changes, or broad runtime activation.

## Planned

### PR-4: “Forget/correct this” narrow path

- **Purpose:** Allow the user to correct or remove explicit memory safely.
- **Exit condition:** forget/correct tests pass; deleted/tombstoned memories are not retrieved; audit/source proof remains; CI green; PR merged.

## Completed

### PR-2: Explicit “remember this” visible memory path

- **Purpose:** Allow Brevio to handle explicit user memory intent in the smallest safe way.
- **Status:** Satisfied on `main` before this queue advance. `rememberVisibleExplicitPreference` exists in `apps/fomo/src/memory/typed-memory-visible-recall.ts`, and targeted tests pass in `apps/fomo/src/memory/typed-memory-visible-recall.test.ts`.
- **Done condition met:** Explicit remember intent is tested; cross-user isolation is tested; source/audit metadata is tested; no migration/new table path is touched; private values are redacted from human-facing output; saved preferences participate in safe visible recall/explain/correct/forget.

### PR-1: Memory V1 phase contract and first visible behavior slice

- **Purpose:** Transition the active phase from M1-B / Memory V1 closeout to Memory V1 Visible Behavior and define the first implementation PR.
- **Status:** Completed in PR #91, canonical commit `f50ede20c7646b80be2844f174b0022afd8b4ef5`, merged as `2e0f9411321c1c42bb97468f59b17574689f8ced`.
- **Done condition met:** active phase says Memory V1 Visible Behavior; exactly one NEXT item remains; first implementation PR was named clearly; verifier passed; CI passed; PR merged; local main synced.

### M1-B / Memory V1 foundation closeout

- **Purpose:** Build the no-migration memory foundation and prove the first trust loop primitives before moving into visible behavior.
- **Status:** Closed by founder decision. Do not reopen M1 A/B/C or continue hidden M1-B hardening unless a specific blocker appears.
- **Evidence:** typed memory validation, retrieval/context pack proof, retrieval evidence helper, no-leak audit metadata, explicit preference memory, why-used explanation, forget/correct behavior, remember-this helper/tests, and queue/exit-condition harness.

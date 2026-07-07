# Brevio Next PR Queue

Harness anchor loaded: BREVIO-HARNESS-V1-NO-CIRCLING-FAST-SHIPPING

There must be exactly one item marked `NEXT`. No cycle may say “continue M1” or “continue Memory V1” vaguely. Move the marker only after the current item is merged or explicitly blocked with owner/action.

## NEXT — PR-2: Explicit “remember this” visible memory path

- **Exact branch:** `memory-v1-remember-this-visible-path`
- **Purpose:** Allow Brevio to handle explicit user memory intent in the smallest safe way. Example behavior: when a user says something like “remember that I prefer short summaries,” Brevio stores or routes that as an explicit preference memory with source/audit metadata.
- **Memory V1 Visible Behavior exit condition advanced:**
  1. remember explicit user preferences;
  2. retrieve relevant memories safely;
  5. prove source/audit metadata;
  6. prevent cross-user leakage;
  7. expose at least one visible memory behavior to the user.
- **Allowed files/areas:**
  - explicit command/intent parsing for explicit memory only;
  - typed memory or existing `memory_signals` path;
  - source/audit metadata;
  - targeted tests;
  - narrow dormant helpers only when directly tied to this visible behavior.
- **Forbidden files/areas:**
  - implicit memory from all messages;
  - autonomous background memory writes;
  - broad runtime ranking changes;
  - DB migrations;
  - new tables/schema changes;
  - production deploy;
  - Calendar live activation;
  - Composio runtime;
  - Tool Gateway;
  - browser automation/action tools;
  - broad HMR rewrite;
  - OAuth/security scope changes.
- **Expected changed files:** a small implementation/test slice in the existing memory-visible behavior area; no broad docs/harness changes.
- **Tests required:** explicit remember intent test; cross-user isolation test; source/audit metadata test; no migration/new table proof; no private value leak proof; CI for exact PR commit.
- **Merge condition:** PR exists, CI green for exact PR commit, diff stays inside approved Memory V1 visible behavior scope, no forbidden surfaces touched.
- **Exit condition:** PR merged and local `main` synced; explicit remember intent is proven by tests and the saved preference can participate in safe visible recall.
- **Stop condition:** Stop and report owner/action if implementation requires migration/new table, implicit/global memory capture, production deploy, OAuth/security scope change, or activation of Calendar/Composio/Tool Gateway/browser/action tools.
- **Founder approval needed?** No for narrow helper/test-level explicit memory behavior; yes before production deploy, new external scopes, irreversible data changes, or broad runtime activation.

## Planned

### PR-3: “Why did you remember/use that?” explanation

- **Purpose:** Expose a small explanation path so Brevio can explain memory use.
- **Exit condition:** user-facing explanation is human-readable; no private audit leakage; tests pass; CI green; PR merged.

### PR-4: “Forget/correct this” narrow path

- **Purpose:** Allow the user to correct or remove explicit memory safely.
- **Exit condition:** forget/correct tests pass; deleted/tombstoned memories are not retrieved; audit/source proof remains; CI green; PR merged.

## Completed

### PR-1: Memory V1 phase contract and first visible behavior slice

- **Purpose:** Transition the active phase from M1-B / Memory V1 closeout to Memory V1 Visible Behavior and define the first implementation PR.
- **Status:** In progress in the current phase/queue-only PR.
- **Done condition:** active phase says Memory V1 Visible Behavior; exactly one NEXT item remains; first implementation PR is named clearly; verifier passes; CI passes; PR merged; local main synced.

### M1-B / Memory V1 foundation closeout

- **Purpose:** Build the no-migration memory foundation and prove the first trust loop primitives before moving into visible behavior.
- **Status:** Closed by founder decision. Do not reopen M1 A/B/C or continue hidden M1-B hardening unless a specific blocker appears.
- **Evidence:** typed memory validation, retrieval/context pack proof, retrieval evidence helper, no-leak audit metadata, explicit preference memory, why-used explanation, forget/correct behavior, remember-this helper/tests, and queue/exit-condition harness.

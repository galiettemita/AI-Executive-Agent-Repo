# Brevio Next PR Queue

Harness anchor loaded: BREVIO-HARNESS-V1-NO-CIRCLING-FAST-SHIPPING

There must be exactly one item marked `NEXT`. No cycle may say “continue M1” or “continue Memory V1” vaguely. Move the marker only after the current item is merged or explicitly blocked with owner/action.

## NEXT — PR-12: Unified visible memory command caller seam

- **Exact branch:** `memory-v1-visible-memory-unified-caller-seam`
- **Purpose:** Add the smallest safe dormant caller seam that accepts explicit memory-command text plus caller-supplied parsed remember/query/correction context and routes remember/review/explain/forget/correct through the existing visible memory helpers, without activating a live provider path or parsing arbitrary private text into memory.
- **Memory V1 Visible Behavior exit condition advanced:**
  1. remember explicit user preferences;
  2. retrieve relevant memories safely;
  3. explain why a memory was used;
  4. forget or correct a memory;
  5. prove source/audit metadata;
  6. prevent cross-user leakage;
  7. expose at least one visible memory behavior to the user.
- **Allowed files/areas:**
  - narrow dormant integration seam/type surface around `rememberVisibleExplicitPreferenceFromCaller` and `routeVisibleMemoryCommandFromCaller`;
  - tests proving remember commands require caller-supplied parsed preference context before any write;
  - tests proving review/explain/forget/correct commands require their existing explicit caller context where destructive or corrective;
  - tests proving unknown/non-memory text and missing parsed context are no-ops;
  - tests proving source/audit metadata is safe and private raw values/source refs are excluded from user-visible/audit-safe output;
  - tests proving user scoping and no cross-user leakage;
  - narrow queue advancement after PR-11.
- **Forbidden files/areas:**
  - production/runtime activation beyond a dormant integration seam;
  - freeform LLM parsing of arbitrary user text into persisted memory;
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
- **Expected changed files:** a small implementation/test slice in the existing FOMO memory-visible behavior area; no broad docs/harness changes except queue advancement after PR-11.
- **Tests required:** new unified caller seam tests pass; existing visible memory remember/recall/review/explain/forget/correct/router/caller tests still pass; full lint/test/build and CI for exact PR commit.
- **Merge condition:** PR exists, CI green for exact PR commit, diff stays inside approved Memory V1 visible behavior scope, no forbidden surfaces touched.
- **Exit condition:** PR merged and local `main` synced; an external caller can use one dormant visible-memory command seam for remember/review/explain/forget/correct only with explicit caller-supplied context, without live activation or private/cross-user leakage.
- **Stop condition:** Stop and report owner/action if implementation requires migration/new table, production deploy, OAuth/security scope change, freeform LLM memory parsing, or activation of Calendar/Composio/Tool Gateway/browser/action tools.
- **Founder approval needed?** No for a narrow dormant helper/test-level integration seam; yes before production deploy, new external scopes, irreversible data changes, freeform memory extraction/parsing, or broad runtime activation.

## Completed

### PR-11: Visible memory remember command caller seam

- **Exact branch:** `memory-v1-visible-memory-remember-caller-seam`
- **Purpose:** Add the smallest safe dormant caller seam for explicit “remember this” memory intent, where an outside caller supplies already-parsed preference attribute/value/source context and the seam delegates to the existing visible explicit-preference remember helper without activating a live provider path or parsing arbitrary private text into memory.
- **Memory V1 Visible Behavior exit condition advanced:**
  1. remember explicit user preferences;
  5. prove source/audit metadata;
  6. prevent cross-user leakage;
  7. expose at least one visible memory behavior to the user.
- **Allowed files/areas:**
  - narrow dormant integration seam/type surface around `rememberVisibleExplicitPreference`;
  - tests proving caller-supplied parsed preference context is required before any write;
  - tests proving unknown/non-memory text, review/explain/forget/correct text, and missing parsed preference context are no-ops;
  - tests proving source/audit metadata is safe and private raw values/source refs are excluded from user-visible/audit-safe output;
  - tests proving user scoping and no cross-user leakage;
  - narrow queue advancement after PR-10.
- **Forbidden files/areas:**
  - production/runtime activation beyond a dormant integration seam;
  - freeform LLM parsing of arbitrary user text into persisted memory;
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
- **Expected changed files:** a small implementation/test slice in the existing FOMO memory-visible behavior area; no broad docs/harness changes except queue advancement after PR-10.
- **Tests required:** new remember caller seam tests pass; existing visible memory remember/recall/review/explain/forget/correct/router tests still pass; full lint/test/build and CI for exact PR commit.
- **Merge condition:** PR exists, CI green for exact PR commit, diff stays inside approved Memory V1 visible behavior scope, no forbidden surfaces touched.
- **Exit condition:** PR merged and local `main` synced; an external caller can use a dormant remember-this seam only with explicit caller-supplied parsed preference context, without live activation or private/cross-user leakage.
- **Stop condition:** Stop and report owner/action if implementation requires migration/new table, production deploy, OAuth/security scope change, freeform LLM memory parsing, or activation of Calendar/Composio/Tool Gateway/browser/action tools.
- **Founder approval needed?** No for a narrow dormant helper/test-level integration seam; yes before production deploy, new external scopes, irreversible data changes, freeform memory extraction/parsing, or broad runtime activation.
- **Status:** Completed in PR #107, branch `memory-v1-visible-memory-remember-caller-seam`, canonical commit `1aac91334f48192aa1f9bcb806e898bbcc366406`, merged as `c18caee5ed27f50230f4c1f263f072f67903cdc2`.
- **Done condition met:** `rememberVisibleExplicitPreferenceFromCaller`, `VisibleMemoryRememberCallerContext`, `VisibleMemoryRememberCommandResult`, and `isVisibleMemoryRememberCommandText` exist in `apps/fomo/src/memory/typed-memory-visible-recall.ts`; targeted tests prove caller-supplied parsed preference context is required before writing; unknown/non-memory text, review/explain/forget/correct text, and missing parsed preference context are no-ops; user scoping and no cross-user leakage are preserved; private values and raw source refs do not leak; CI passed for head commit `1aac9133`; PR merged; local main synced.

### PR-10: Visible memory command router integration seam

- **Exact branch:** `memory-v1-visible-memory-router-integration-seam`
- **Purpose:** Add the smallest safe dormant integration seam that lets a caller pass explicit user memory-command text plus caller-supplied query/correction context into the visible-memory command router from outside the memory module, without activating a live provider path or changing production behavior.
- **Memory V1 Visible Behavior exit condition advanced:**
  2. retrieve relevant memories safely;
  3. explain why a memory was used;
  4. forget or correct a memory;
  5. prove source/audit metadata;
  6. prevent cross-user leakage;
  7. expose at least one visible memory behavior to the user.
- **Allowed files/areas:**
  - narrow dormant integration seam/type surface around the existing visible memory command router;
  - tests proving the seam delegates review/explain/forget/correct commands to the router with explicit caller-supplied context;
  - tests proving unknown/non-memory text is a no-op and destructive commands require explicit query/correction context;
  - tests proving unsafe/inactive/cross-user memories and private raw values/source refs remain excluded;
  - narrow docs/queue advancement after PR-9.
- **Forbidden files/areas:**
  - production/runtime activation beyond a dormant integration seam;
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
- **Expected changed files:** a small implementation/test slice in the existing FOMO memory-visible behavior area; no broad docs/harness changes except queue advancement after PR-9.
- **Tests required:** new seam tests pass; existing visible memory router/remember/recall/review/explain/forget/correct tests still pass; full lint/test/build and CI for exact PR commit.
- **Merge condition:** PR exists, CI green for exact PR commit, diff stays inside approved Memory V1 visible behavior scope, no forbidden surfaces touched.
- **Exit condition:** PR merged and local `main` synced; an external caller can use the dormant router seam safely without live activation or private/cross-user leakage.
- **Stop condition:** Stop and report owner/action if implementation requires migration/new table, production deploy, OAuth/security scope change, or activation of Calendar/Composio/Tool Gateway/browser/action tools.
- **Status:** Completed in PR #106, branch `memory-v1-visible-memory-router-integration-seam`, canonical commit `edf3cb7941e30b20ea2960ebdc4cfe46e2e80dd1`, merged as `e93608de6232dd7131cab4092b3f6cdf03601c4d`.
- **Done condition met:** `routeVisibleMemoryCommandFromCaller` and `VisibleMemoryCommandCallerContext` exist in `apps/fomo/src/memory/typed-memory-visible-recall.ts`; targeted tests prove caller context delegates review/explain/forget/correct commands to the router with explicit query/correction context; unknown and remember-this text remain no-ops; destructive commands require explicit caller-supplied context; inactive, stale, retracted, tombstoned, low-confidence, and cross-user memories remain excluded; private values and raw source refs do not leak; CI passed for head commit `edf3cb79`; PR merged; local main synced.

### PR-9: Visible memory command router for explicit preferences

- **Purpose:** Add the smallest safe dormant router that receives a user memory-command text plus explicit caller-supplied query/options and routes to the existing visible explicit-preference review, explanation, forget, and correct command adapters without live provider activation or exposing private raw values/cross-user data.
- **Status:** Completed in PR #104, branch `memory-v1-visible-memory-command-router`, canonical commit `5b64a4d03b9f31b3c080609eaa17174ef435025d`, merged as `504f4a9036bd78af063835a53470b11076d36cfd`.
- **Done condition met:** `routeVisibleMemoryCommand` exists in `apps/fomo/src/memory/typed-memory-visible-recall.ts`; targeted tests prove review/explain/forget/correct command texts route to existing helper outputs when required query/options are supplied; unknown and remember-this text do not trigger writes/retractions; correction requires correction options; inactive, stale, retracted, tombstoned, low-confidence, and cross-user memories remain excluded; private values and raw source refs do not leak; CI passed for head commit `5b64a4d0`; PR merged; local main synced.

### PR-8: “Forget/correct that” command adapter for explicit preferences

- **Purpose:** Add the smallest safe dormant command-adapter path that recognizes a user asking Brevio to forget or correct a saved explicit preference and routes to the existing visible explicit-preference forget/correct helpers, without live provider activation or exposing private raw values/cross-user data.
- **Status:** Completed in PR #103, branch `memory-v1-visible-memory-forget-correct-command-adapter`, canonical commit `5076f3d2e07eaeb269c805b31d938ce97dd41fa4`, merged as `2e674bc0b439695fc9f033334d0a588ac8ac813d`.
- **Done condition met:** `answerVisibleMemoryForgetCommand`, `answerVisibleMemoryCorrectCommand`, `isVisibleMemoryForgetCommandText`, and `isVisibleMemoryCorrectCommandText` exist in `apps/fomo/src/memory/typed-memory-visible-recall.ts`; targeted tests prove forget-that and correct-that requests route to explicit-preference helper output when an attribute/query is supplied; unknown and remember-this text do not trigger forget/correct; inactive, stale, retracted, tombstoned, low-confidence, and cross-user memories remain excluded; private values and raw source refs do not leak; CI passed for merge commit `2e674bc0`; PR merged; local main synced.

### PR-7: “Why did you remember/use that?” command adapter for explicit-preference explanation

- **Purpose:** Add the smallest safe command-adapter path that recognizes a user asking why Brevio remembered or used a saved preference and returns the existing visible explicit-preference explanation helper output, without live provider activation or exposing private raw values/cross-user data.
- **Status:** Completed in PR #101, canonical commit `22c2d167573fdbda51e14cc1fdffa882d40609c3`, merged as `69005f8b1fe2790f69ead38831460ce1209c7333`.
- **Done condition met:** `answerVisibleMemoryExplanationCommand` and `isVisibleMemoryExplanationCommandText` exist in `apps/fomo/src/memory/typed-memory-visible-recall.ts`; targeted tests prove why-remembered/why-used requests route to explicit-preference explanation output; unknown and remember-this text do not trigger explanation; inactive, stale, retracted, tombstoned, low-confidence, and cross-user memories remain excluded; private values and raw source refs do not leak; CI passed; PR merged; local main synced.

### PR-6: “What do you remember?” command adapter for explicit-preference review

- **Purpose:** Add the smallest safe command-adapter path that recognizes a user asking what Brevio remembers and returns the existing explicit-preference review helper output, without live provider activation or exposing private raw values/cross-user data.
- **Status:** Completed in PR #99, canonical commit `fddafd9f3891ac08bf03ef1af2ffab5d2e77c340`, merged as `15e7d34b79d3c99f548d7c1886fc36e01aa6ea22`.
- **Done condition met:** `answerVisibleMemoryReviewCommand` and `isVisibleMemoryReviewCommandText` exist in `apps/fomo/src/memory/typed-memory-visible-recall.ts`; targeted tests prove review-style requests route to explicit-preference review output; unknown and remember-this text do not trigger review; inactive, stale, retracted, tombstoned, low-confidence, and cross-user memories remain excluded; private values and raw source refs do not leak; CI passed; PR merged; local main synced.

### PR-5: “What do you remember?” explicit-preference review path

- **Purpose:** Let a user review the explicit preferences Brevio can safely remember for them, without exposing private raw values or cross-user data.
- **Status:** Completed in PR #97, canonical commit `a09575e7fbf13e418a4271d929dcf7b0465b8717`, merged as `340e54b4429455f7f82a48a0b63fde82806cc7c5`.
- **Done condition met:** `reviewVisibleExplicitPreferences` exists in `apps/fomo/src/memory/typed-memory-visible-recall.ts`; targeted tests prove active explicit preferences are reviewable; stale/retracted/tombstoned/low-confidence/feedback-derived/cross-user memories are excluded; source/audit metadata is summarized; private values and raw source refs do not leak; CI passed; PR merged; local main synced.

### PR-4: “Forget/correct this” narrow path

- **Purpose:** Allow the user to correct or remove explicit memory safely through the narrow visible Memory V1 path.
- **Status:** Satisfied on `main` before queue advance. `forgetVisibleExplicitPreference` and `correctVisibleExplicitPreference` exist in `apps/fomo/src/memory/typed-memory-visible-recall.ts`, and targeted tests pass in `apps/fomo/src/memory/typed-memory-visible-recall.test.ts`.
- **Done condition met:** visible explicit-preference forget/correct helpers are tested; corrected preferences replace old recall; forgotten preferences stop surfacing; deleted/tombstoned/inactive memories are not retrieved or explained; cross-user isolation is tested; source/audit metadata remains available; private values and raw source refs are not exposed; no migration/new table/runtime activation path is touched.

### PR-3: “Why did you remember/use that?” explanation

- **Purpose:** Expose a small explanation path so Brevio can explain memory use in a human-readable, privacy-safe way.
- **Status:** Completed in PR #94, canonical commit `937a3f44ee100e2ab0f74645c344fe8c1f7f2acd`, merged as `2a4bf612e795564a5f859b627df1006b4bc79300`.
- **Done condition met:** scoped helper explains visible explicit preference memory use; tests prove human-readable explanation, source/audit metadata, cross-user isolation, no private value/source-ref leakage, and no explanation for unsafe inactive memories; CI passed; PR merged; local main synced.

### PR-2: Explicit “remember this” visible memory path

- **Purpose:** Allow Brevio to handle explicit user memory intent in the smallest safe way.
- **Status:** Satisfied on `main` before queue advance. `rememberVisibleExplicitPreference` exists in `apps/fomo/src/memory/typed-memory-visible-recall.ts`, and targeted tests pass in `apps/fomo/src/memory/typed-memory-visible-recall.test.ts`.
- **Done condition met:** Explicit remember intent is tested; cross-user isolation is tested; source/audit metadata is tested; no migration/new table path is touched; private values are redacted from human-facing output; saved preferences participate in safe visible recall/explain/correct/forget.

### PR-1: Memory V1 phase contract and first visible behavior slice

- **Purpose:** Transition the active phase from M1-B / Memory V1 closeout to Memory V1 Visible Behavior and define the first implementation PR.
- **Status:** Completed in PR #91, canonical commit `f50ede20c7646b80be2844f174b0022afd8b4ef5`, merged as `2e0f9411321c1c42bb97468f59b17574689f8ced`.
- **Done condition met:** active phase says Memory V1 Visible Behavior; exactly one NEXT item remains; first implementation PR was named clearly; verifier passed; CI passed; PR merged; local main synced.

### M1-B / Memory V1 foundation closeout

- **Purpose:** Build the no-migration memory foundation and prove the first trust loop primitives before moving into visible behavior.
- **Status:** Closed by founder decision. Do not reopen M1 A/B/C or continue hidden M1-B hardening unless a specific blocker appears.
- **Evidence:** typed memory validation, retrieval/context pack proof, retrieval evidence helper, no-leak audit metadata, explicit preference memory, why-used explanation, forget/correct behavior, remember-this helper/tests, and queue/exit-condition harness.

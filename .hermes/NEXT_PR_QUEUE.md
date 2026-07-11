# Brevio Next PR Queue

Harness anchor loaded: BREVIO-HARNESS-V1-NO-CIRCLING-FAST-SHIPPING

There must be exactly one item marked `NEXT`. No cycle may say “continue M1” or “continue Memory V1” vaguely. Move the marker only after the current item is merged or explicitly blocked with owner/action.

## NEXT — PR-22: Disabled SendBlue visible-memory command reply delivery candidate seam

- **Exact branch:** `memory-v1-sendblue-visible-memory-command-reply-delivery-candidate-disabled-seam`
- **Purpose:** Add the smallest safe disabled-by-default delivery-candidate seam that can convert the PR-21 sanitized visible-memory command reply text into a deterministic caller-inspectable SendBlue delivery candidate for a future approved send step, without sending any outbound message, changing current HTTP responses, enabling production runtime behavior, or exposing raw private memory values/source refs/full phone numbers in logs/audit metadata.
- **Memory V1 Visible Behavior exit condition advanced:**
  1. remember explicit user preferences;
  2. retrieve relevant memories safely;
  3. explain why a memory was used;
  4. forget or correct a memory;
  5. prove source/audit metadata;
  6. prevent cross-user leakage;
  7. expose at least one visible memory behavior to the user.
- **Allowed files/areas:**
  - narrow helper/seam around the existing SendBlue inbound `visibleMemoryCommand.response.replyText` result and inbound `fromNumber` delivery target;
  - tests proving the seam is absent/inert by default and no outbound SendBlue send occurs;
  - tests proving enabled test-level candidate capture uses only sanitized reply text, structural renderer metadata, and a non-logged/non-audited in-memory recipient target;
  - tests proving empty/unhandled reply text produces no delivery candidate;
  - tests proving private memory values/source refs/arbitrary inbound text/cross-user data/full phone numbers do not leak into audit or response metadata;
  - tests proving current public HTTP response shape, STOP/START behavior, existing reply parsing, and why/explain handling are unchanged unless explicitly test-enabled.
- **Forbidden files/areas:**
  - sending any outbound memory command response over SendBlue;
  - production/runtime activation beyond a disabled-by-default seam;
  - changing the existing public HTTP response shape by default;
  - persisting full phone numbers, raw command text, private memory values, or source refs;
  - freeform LLM parsing or rendering of arbitrary user text into memory;
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
- **Expected changed files:** a small implementation/test slice in the existing SendBlue inbound route tests/route and visible-memory reply-text boundary; no broad docs/harness changes except queue advancement after PR-21.
- **Tests required:** targeted SendBlue inbound visible-memory reply delivery-candidate seam tests pass; existing sendblue-inbound/why/explain tests still pass; full lint/test/build and CI for exact PR commit.
- **Merge condition:** PR exists, CI green for exact PR commit, diff stays inside approved Memory V1 visible behavior scope, no forbidden surfaces touched, disabled/default path remains inert, no outbound response is sent, and no full phone/private memory data is persisted or exposed in audit/HTTP metadata.
- **Exit condition:** PR merged and local `main` synced; SendBlue inbound has a disabled-by-default, test-proven delivery-candidate seam that can expose a safe deterministic visible-memory command reply candidate for a future founder-approved delivery PR without live activation, outbound send, private/cross-user leakage, full-phone persistence, or regression of existing reply/STOP/explain behavior.
- **Stop condition:** Stop and report owner/action if implementation requires migration/new table, production deploy, OAuth/security scope change, freeform LLM memory parsing/rendering, raw command text/private values/source refs/full phone numbers in audit/response metadata, live default activation, outbound SendBlue send, or activation of Calendar/Composio/Tool Gateway/browser/action tools.
- **Founder approval needed?** No for a narrow disabled-by-default delivery-candidate seam and tests; yes before production deploy, new external scopes, irreversible data changes, freeform memory extraction/parsing/rendering, raw private audit/response persistence, outbound memory command send, live default activation, or broad runtime activation.

## Completed

### PR-21: Disabled SendBlue visible-memory command reply text renderer seam

- **Exact branch:** `memory-v1-sendblue-visible-memory-command-reply-renderer-disabled-seam`
- **Purpose:** Add the smallest safe disabled-by-default renderer seam that converts the existing sanitized visible-memory command adapter result envelope into deterministic reply text for future SendBlue delivery, without sending any outbound message, changing current HTTP responses, enabling production runtime behavior, or exposing raw private memory values/source refs in logs/audit metadata.
- **Memory V1 Visible Behavior exit condition advanced:**
  1. remember explicit user preferences;
  2. retrieve relevant memories safely;
  3. explain why a memory was used;
  4. forget or correct a memory;
  5. prove source/audit metadata;
  6. prevent cross-user leakage;
  7. expose at least one visible memory behavior to the user.
- **Allowed files/areas:**
  - narrow renderer/helper around the existing SendBlue inbound `visibleMemoryCommand.response` envelope or app-adapter result types;
  - tests proving the renderer is absent/inert by default and no outbound SendBlue send occurs;
  - tests proving enabled test-level rendering for remember/review/explain/forget/correct statuses uses only allowed sanitized fields and safe response text already produced by the app adapter;
  - tests proving private memory values/source refs/arbitrary inbound text/cross-user data do not leak into audit or response metadata;
  - tests proving current public HTTP response shape, STOP/START behavior, existing reply parsing, and why/explain handling are unchanged unless explicitly test-enabled.
- **Forbidden files/areas:**
  - sending any outbound memory command response over SendBlue;
  - production/runtime activation beyond a disabled-by-default seam;
  - changing the existing public HTTP response shape by default;
  - freeform LLM parsing or rendering of arbitrary user text into memory;
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
- **Expected changed files:** a small implementation/test slice in the existing SendBlue inbound route tests/route and visible-memory adapter boundary; no broad docs/harness changes except queue advancement after PR-20.
- **Tests required:** targeted SendBlue inbound visible-memory reply-renderer seam tests pass; existing sendblue-inbound/why/explain tests still pass; full lint/test/build and CI for exact PR commit.
- **Merge condition:** PR exists, CI green for exact PR commit, diff stays inside approved Memory V1 visible behavior scope, no forbidden surfaces touched, disabled/default path remains inert and no outbound response is sent.
- **Exit condition:** PR merged and local `main` synced; SendBlue inbound has a disabled-by-default, test-proven renderer seam that can produce safe deterministic visible-memory command reply text for a future delivery PR without live activation, outbound send, private/cross-user leakage, or regression of existing reply/STOP/explain behavior.
- **Stop condition:** Stop and report owner/action if implementation requires migration/new table, production deploy, OAuth/security scope change, freeform LLM memory parsing/rendering, raw command text/private values in audit/response metadata, live default activation, outbound SendBlue send, or activation of Calendar/Composio/Tool Gateway/browser/action tools.
- **Founder approval needed?** No for a narrow disabled-by-default renderer seam and tests; yes before production deploy, new external scopes, irreversible data changes, freeform memory extraction/parsing/rendering, raw private audit/response persistence, outbound memory command send, live default activation, or broad runtime activation.
- **Status:** Completed in PR #124, branch `memory-v1-sendblue-visible-memory-command-reply-renderer-disabled-seam`, canonical commit `2efcc163c8f4dafd0890aa18fc36dfc632374010`, merged as `0e12ad0b4f6baa9a46309b9d0acf607b42333923`.
- **Done condition met:** `SendBlueInboundVisibleMemoryCommandReplyText`, `SendBlueInboundVisibleMemoryCommandReplyTextRendererOptions`, and `renderSendBlueInboundVisibleMemoryCommandReplyText` exist in `apps/fomo/src/routes/sendblue-inbound.ts`; targeted tests prove the reply-text renderer helper uses only sanitized app-adapter response text and structural metadata, is inert by default, renders test-enabled remember/review/explain/correct/forget statuses, preserves the public HTTP response shape, and excludes private/cross-user values/source refs/raw command refs from rendered metadata; CI passed for head commit `2efcc163`; PR merged as `0e12ad0b`; local main synced.

### PR-20: Disabled SendBlue inbound visible memory exact-command context parser seam

- **Exact branch:** `memory-v1-sendblue-inbound-visible-memory-exact-command-context-parser-disabled-seam`
- **Purpose:** Add the smallest safe disabled-by-default SendBlue inbound context parser seam for explicit visible-memory commands, so future runtime wiring can convert only exact user-command forms such as remember/review/explain/forget/correct into caller-provided `visibleMemoryCommand` context without LLM parsing, production activation, outbound sends, or changing current HTTP responses.
- **Memory V1 Visible Behavior exit condition advanced:**
  1. remember explicit user preferences;
  2. retrieve relevant memories safely;
  3. explain why a memory was used;
  4. forget or correct a memory;
  5. prove source/audit metadata;
  6. prevent cross-user leakage;
  7. expose at least one visible memory behavior to the user.
- **Allowed files/areas:**
  - narrow disabled-by-default exact-command parser/helper around the existing SendBlue inbound `visibleMemoryCommand.contextResolver` boundary;
  - tests proving the parser is absent/inert by default and preserves current HTTP response shape;
  - tests proving enabled test-level parser accepts only explicit command prefixes/forms and returns typed caller context for remember/review/explain/forget/correct;
  - tests proving non-command, ambiguous, STOP/START, existing reply parsing, why/explain handling, state transitions, and sanitized audit behavior are not regressed;
  - tests proving no outbound SendBlue send occurs and no production runtime activation occurs;
  - tests proving private values/source refs/arbitrary non-command inbound text/cross-user data do not leak through audit or response metadata.
- **Forbidden files/areas:**
  - production/runtime activation beyond a disabled-by-default seam;
  - sending any outbound memory command response over SendBlue;
  - changing the existing public HTTP response shape unless the seam is explicitly test-enabled;
  - freeform LLM parsing of arbitrary user text into persisted memory;
  - parsing non-command arbitrary inbound text into memory context;
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
- **Expected changed files:** a small implementation/test slice in the existing SendBlue inbound route tests/route and visible-memory adapter boundary; no broad docs/harness changes except queue advancement after PR-19.
- **Tests required:** targeted SendBlue inbound visible-memory exact-command parser seam tests pass; existing sendblue-inbound/why/explain tests still pass; existing typed-memory visible-recall tests still pass if adapter types change; full lint/test/build and CI for exact PR commit.
- **Merge condition:** PR exists, CI green for exact PR commit, diff stays inside approved Memory V1 visible behavior scope, no forbidden surfaces touched, disabled/default path remains inert and no outbound response is sent.
- **Exit condition:** PR merged and local `main` synced; SendBlue inbound has a disabled-by-default, test-proven exact-command context parser seam that can feed the visible-memory command adapter only explicit memory command context without live activation, outbound send, private/cross-user leakage, or regression of existing reply/STOP/explain behavior.
- **Stop condition:** Stop and report owner/action if implementation requires migration/new table, production deploy, OAuth/security scope change, freeform LLM memory parsing, raw private values in audit/response metadata, live default activation, outbound SendBlue send, or activation of Calendar/Composio/Tool Gateway/browser/action tools.
- **Founder approval needed?** No for a narrow disabled-by-default exact-command parser seam and tests; yes before production deploy, new external scopes, irreversible data changes, freeform memory extraction/parsing, raw private audit/response persistence, outbound memory command send, live default activation, or broad runtime activation.
- **Status:** Completed in PR #122, branch `memory-v1-sendblue-inbound-visible-memory-exact-command-context-parser-disabled-seam`, canonical commit `ed32e07e6bbc776aee32323ce2bd5bc9350ca705`, merged as `af382ff063c3a48a53f230757e2e5dbc918bcea8`.
- **Done condition met:** `SendBlueInboundVisibleMemoryCommandContextParser`, `SendBlueInboundVisibleMemoryCommandContextParserOptions`, and `parseSendBlueInboundVisibleMemoryExactCommandContext` exist in `apps/fomo/src/routes/sendblue-inbound.ts`; targeted tests prove exact remember/review/explain/forget/correct forms, non-command rejection, disabled/inert default behavior, STOP/START bypass, current HTTP response shape preservation, sanitized audit/response metadata, and cross-user/private leak exclusion; CI passed for head commit `ed32e07e`; PR merged as `af382ff0`; local main synced.

### PR-19: Disabled SendBlue inbound visible memory command response seam

- **Exact branch:** `memory-v1-sendblue-inbound-visible-memory-command-response-disabled-seam`
- **Purpose:** Add the smallest safe disabled-by-default SendBlue inbound response seam that can capture the visible-memory command app adapter result in a deterministic, caller-inspectable envelope for future reply rendering, without sending any outbound message, changing current HTTP responses, parsing arbitrary private inbound text into memory, or changing production behavior.
- **Memory V1 Visible Behavior exit condition advanced:**
  1. remember explicit user preferences;
  2. retrieve relevant memories safely;
  3. explain why a memory was used;
  4. forget or correct a memory;
  5. prove source/audit metadata;
  6. prevent cross-user leakage;
  7. expose at least one visible memory behavior to the user.
- **Allowed files/areas:**
  - narrow disabled-by-default SendBlue inbound visible-memory response type/helper around the existing PR-17/PR-18 `visibleMemoryCommand` boundary;
  - tests proving the response seam default is absent/inert and preserves current HTTP response shape;
  - tests proving enabled test-level seam captures only sanitized structural adapter result metadata and safe user-facing response text from the app adapter result;
  - tests proving no outbound SendBlue send occurs and no production runtime activation occurs;
  - tests proving STOP/START, existing reply parsing, why/explain handling, state transitions, and sanitized audit behavior are not regressed;
  - tests proving private values/source refs/arbitrary inbound text/cross-user data do not leak through response metadata;
  - narrow queue advancement after PR-18.
- **Forbidden files/areas:**
  - production/runtime activation beyond a disabled-by-default seam;
  - sending any outbound memory command response over SendBlue;
  - changing the existing public HTTP response shape unless the seam is explicitly test-enabled;
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
- **Expected changed files:** a small implementation/test slice in the existing SendBlue inbound route tests/route and visible-memory adapter boundary; no broad docs/harness changes except queue advancement after PR-18.
- **Tests required:** targeted SendBlue inbound visible-memory response seam tests pass; existing sendblue-inbound/why/explain tests still pass; existing typed-memory visible-recall tests still pass if adapter types change; full lint/test/build and CI for exact PR commit.
- **Merge condition:** PR exists, CI green for exact PR commit, diff stays inside approved Memory V1 visible behavior scope, no forbidden surfaces touched, disabled/default path remains inert and no outbound response is sent.
- **Exit condition:** PR merged and local `main` synced; SendBlue inbound has a disabled-by-default, test-proven response seam that can expose a sanitized visible-memory command adapter result to a future caller without live activation, outbound send, private/cross-user leakage, or regression of existing reply/STOP/explain behavior.
- **Stop condition:** Stop and report owner/action if implementation requires migration/new table, production deploy, OAuth/security scope change, freeform LLM memory parsing, raw command text/private values in audit/response metadata, live default activation, outbound SendBlue send, or activation of Calendar/Composio/Tool Gateway/browser/action tools.
- **Founder approval needed?** No for a narrow disabled-by-default response seam and tests; yes before production deploy, new external scopes, irreversible data changes, freeform memory extraction/parsing, raw private audit/response persistence, outbound memory command send, live default activation, or broad runtime activation.
- **Status:** Completed in PR #120, branch `memory-v1-sendblue-inbound-visible-memory-command-response-disabled-seam`, canonical commit `9891b5c9eae997eb3636098fc179b0b31073362f`, merged as `00c031b230c6095696f320ce949e9741e16c96cf`.
- **Done condition met:** `SendBlueInboundVisibleMemoryCommandResponseEnvelope`, `SendBlueInboundVisibleMemoryCommandResponseRecorder`, `SendBlueInboundVisibleMemoryCommandResponseOptions`, and `visibleMemoryCommand.response` exist in `apps/fomo/src/routes/sendblue-inbound.ts`; targeted tests prove the response seam is absent/inert by default, enabled test-level response capture records only sanitized structural adapter metadata and safe response text, the public HTTP response shape is preserved, STOP/START do not capture responses, user scoping and private/cross-user leak exclusion are preserved, no outbound SendBlue send is wired, CI passed for head commit `9891b5c9`; PR merged as `00c031b2`; local main synced.

### PR-18: Disabled SendBlue inbound visible memory command context resolver seam

- **Exact branch:** `memory-v1-sendblue-inbound-visible-memory-command-context-resolver-disabled-seam`
- **Purpose:** Add the smallest safe disabled-by-default resolver seam that can supply deterministic, caller-provided visible-memory command context to the PR-17 SendBlue inbound adapter integration, so a future runtime caller has one explicit boundary for remember/review/explain/forget/correct context without parsing arbitrary private inbound text into memory or changing current production behavior.
- **Memory V1 Visible Behavior exit condition advanced:**
  1. remember explicit user preferences;
  2. retrieve relevant memories safely;
  3. explain why a memory was used;
  4. forget or correct a memory;
  5. prove source/audit metadata;
  6. prevent cross-user leakage;
  7. expose at least one visible memory behavior to the user.
- **Allowed files/areas:**
  - narrow disabled-by-default SendBlue inbound visible-memory context resolver type/helper around the existing PR-17 `visibleMemoryCommand.context` boundary;
  - tests proving the resolver default is absent/inert and cannot read/write/retract memory;
  - tests proving enabled test-level resolver returns only explicit caller-supplied parsed remember/query/correction context;
  - tests proving resolver output never derives persisted memory values from arbitrary inbound freeform text;
  - tests proving STOP/START, existing reply parsing, why/explain handling, state transitions, and sanitized audit behavior are not regressed;
  - tests proving user scoping, private-value/source-ref redaction, and no cross-user leakage;
  - narrow queue advancement after PR-17.
- **Forbidden files/areas:**
  - production/runtime activation beyond a disabled-by-default seam;
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
- **Expected changed files:** a small implementation/test slice in the existing SendBlue inbound route tests/route and visible-memory adapter boundary; no broad docs/harness changes except queue advancement after PR-17.
- **Tests required:** targeted SendBlue inbound visible-memory context resolver tests pass; existing sendblue-inbound/why/explain tests still pass; existing typed-memory visible-recall tests still pass if adapter types change; full lint/test/build and CI for exact PR commit.
- **Merge condition:** PR exists, CI green for exact PR commit, diff stays inside approved Memory V1 visible behavior scope, no forbidden surfaces touched, disabled/default path remains inert.
- **Exit condition:** PR merged and local `main` synced; SendBlue inbound has a disabled-by-default, test-proven context resolver seam feeding the visible-memory command adapter only explicit caller-supplied context without live activation, private/cross-user leakage, or regression of existing reply/STOP/explain behavior.
- **Stop condition:** Stop and report owner/action if implementation requires migration/new table, production deploy, OAuth/security scope change, freeform LLM memory parsing, raw command text/private values in audit, live default activation, or activation of Calendar/Composio/Tool Gateway/browser/action tools.
- **Founder approval needed?** No for a narrow disabled-by-default resolver seam and tests; yes before production deploy, new external scopes, irreversible data changes, freeform memory extraction/parsing, raw private audit persistence, live default activation, or broad runtime activation.
- **Status:** Completed in PR #119, branch `memory-v1-sendblue-inbound-visible-memory-command-context-resolver-disabled-seam`, canonical commit `6e0987dd5990b74829bec16f88dd26494099ebf7`, merged as `a928f182dbf10a3e61de835abd3cf14505fe14f6`.
- **Done condition met:** `SendBlueInboundVisibleMemoryCommandContextResolver`, `SendBlueInboundVisibleMemoryCommandContextResolverInput`, and `visibleMemoryCommand.contextResolver` exist in `apps/fomo/src/routes/sendblue-inbound.ts`; targeted tests prove missing context/resolver remains inert, enabled resolver receives only user id/parsed intent/time and not inbound freeform text, resolver-supplied remember/query/correction context is caller-provided, STOP/START do not resolve context, user scoping and private/cross-user leak exclusion are preserved, and sanitized audit metadata excludes private values/source refs/freeform inbound text; CI passed for head commit `6e0987dd`; PR merged as `a928f182`; local main synced.

### PR-17: Disabled SendBlue inbound visible memory command adapter integration seam

- **Exact branch:** `memory-v1-sendblue-inbound-visible-memory-command-adapter-disabled-seam`
- **Purpose:** Add the smallest safe disabled-by-default integration seam that lets the SendBlue inbound route call the existing visible-memory command app adapter only when explicitly enabled and supplied deterministic/caller-provided memory command context, so Memory V1 can move toward a real user-visible memory command surface without changing current production behavior, parsing arbitrary private text into memory, or bypassing existing STOP/reply-parser/explain behavior.
- **Memory V1 Visible Behavior exit condition advanced:**
  1. remember explicit user preferences;
  2. retrieve relevant memories safely;
  3. explain why a memory was used;
  4. forget or correct a memory;
  5. prove source/audit metadata;
  6. prevent cross-user leakage;
  7. expose at least one visible memory behavior to the user.
- **Allowed files/areas:**
  - narrow disabled-by-default SendBlue inbound/app route integration seam around the existing `handleVisibleMemoryCommandAppAdapterRequest` helper;
  - tests proving the default path is bit-compatible/inert when the seam is disabled or deps are absent;
  - tests proving enabled test-level seam calls the adapter only with explicit caller-supplied parsed/query/correction context, not freeform LLM memory extraction;
  - tests proving STOP, existing reply parsing, why/explain handling, state transitions, and sanitized audit behavior are not regressed;
  - tests proving user scoping, private-value/source-ref redaction, and no cross-user leakage;
  - narrow queue advancement after PR-16.
- **Forbidden files/areas:**
  - production/runtime activation beyond a disabled-by-default seam;
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
- **Expected changed files:** a small implementation/test slice in the existing SendBlue inbound route tests/route and visible-memory adapter boundary; no broad docs/harness changes except queue advancement after PR-16.
- **Tests required:** targeted SendBlue inbound visible-memory seam tests pass; existing sendblue-inbound/why/explain tests still pass; existing typed-memory visible-recall tests still pass if adapter types change; full lint/test/build and CI for exact PR commit.
- **Merge condition:** PR exists, CI green for exact PR commit, diff stays inside approved Memory V1 visible behavior scope, no forbidden surfaces touched, disabled/default path remains inert.
- **Exit condition:** PR merged and local `main` synced; SendBlue inbound has a disabled-by-default, test-proven seam into the visible-memory command adapter without live activation, private/cross-user leakage, or regression of existing reply/STOP/explain behavior.
- **Stop condition:** Stop and report owner/action if implementation requires migration/new table, production deploy, OAuth/security scope change, freeform LLM memory parsing, raw command text/private values in audit, live default activation, or activation of Calendar/Composio/Tool Gateway/browser/action tools.
- **Founder approval needed?** No for a narrow disabled-by-default route integration seam and tests; yes before production deploy, new external scopes, irreversible data changes, freeform memory extraction/parsing, raw private audit persistence, live default activation, or broad runtime activation.
- **Status:** Completed in PR #117, branch `memory-v1-sendblue-inbound-visible-memory-command-adapter-disabled-seam`, canonical commit `8ae01a1428fb317421227df126092990b650e3fd`, merged as `1fc97fcadd7a3aee68afb5563e5eae947c412830`.
- **Done condition met:** `SendBlueInboundRouteDeps.visibleMemoryCommand`, `SendBlueInboundVisibleMemoryCommandContext`, and the disabled-by-default `maybeHandleVisibleMemoryCommand` seam exist in `apps/fomo/src/routes/sendblue-inbound.ts`; targeted tests prove disabled/default inert behavior, enabled caller-supplied parsed context routing, STOP no-invocation behavior, user scoping, no cross-user leakage, and sanitized audit metadata; CI passed for head commit `8ae01a14`; PR merged as `1fc97fca`; local main synced.

### PR-16: Disabled visible memory command app adapter audit-store recorder seam

- **Exact branch:** `memory-v1-visible-memory-command-app-adapter-audit-store-disabled-seam`
- **Purpose:** Add the smallest safe disabled-by-default recorder helper that adapts `VisibleMemoryCommandAppAdapterAuditEvent` into the existing FOMO audit store shape, so a future runtime caller can opt into writing sanitized visible-memory command outcome audit rows through one stable store bridge without activating any live provider path, persisting raw private memory values/source refs, parsing arbitrary private text into memory, or changing current user-facing behavior.
- **Memory V1 Visible Behavior exit condition advanced:**
  1. remember explicit user preferences;
  2. retrieve relevant memories safely;
  3. explain why a memory was used;
  4. forget or correct a memory;
  5. prove source/audit metadata;
  6. prevent cross-user leakage;
  7. expose at least one visible memory behavior to the user.
- **Allowed files/areas:**
  - narrow disabled-by-default audit-store recorder/helper/type surface around the existing `VisibleMemoryCommandAppAdapterAuditEvent` seam;
  - tests proving the recorder default is disabled and does not write audit events;
  - tests proving enabled recorder writes only sanitized structural adapter outcome metadata to the existing audit store shape;
  - tests proving unknown/non-memory text and missing parsed context remain no-ops with safe audit status only;
  - tests proving audit metadata excludes private raw values, source refs, arbitrary command text, and cross-user data;
  - tests proving user scoping and no cross-user leakage;
  - narrow queue advancement after PR-15.
- **Forbidden files/areas:**
  - production/runtime activation beyond a disabled test-level seam;
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
- **Expected changed files:** a small implementation/test slice in the existing FOMO memory-visible behavior and audit type area; no broad docs/harness changes except queue advancement after PR-15.
- **Tests required:** new disabled audit-store recorder seam tests pass; existing visible memory remember/recall/review/explain/forget/correct/router/caller/unified caller/handler/app-adapter/audit-event tests still pass; full lint/test/build and CI for exact PR commit.
- **Merge condition:** PR exists, CI green for exact PR commit, diff stays inside approved Memory V1 visible behavior scope, no forbidden surfaces touched.
- **Exit condition:** PR merged and local `main` synced; a future app/runtime caller can opt into one disabled-by-default audit-store recorder for visible-memory command outcomes, writing only sanitized structural metadata without live activation or private/cross-user leakage.
- **Stop condition:** Stop and report owner/action if implementation requires migration/new table, production deploy, OAuth/security scope change, freeform LLM memory parsing, raw command text/private values in audit, or activation of Calendar/Composio/Tool Gateway/browser/action tools.
- **Founder approval needed?** No for a narrow disabled-by-default audit-store helper/test-level seam; yes before production deploy, new external scopes, irreversible data changes, freeform memory extraction/parsing, raw private audit persistence, or broad runtime activation.
- **Status:** Completed in PR #115, branch `memory-v1-visible-memory-command-app-adapter-audit-store-disabled-seam`, canonical commit `2faee860a42a0208e1829b7e6271780874f738f5`, merged as `8e16a709ed0848e4806bb7e64243c1e342780fd9`.
- **Done condition met:** `createVisibleMemoryCommandAppAdapterAuditStoreRecorder` and `VisibleMemoryCommandAppAdapterAuditStoreRecorderOptions` exist in `apps/fomo/src/memory/typed-memory-visible-recall.ts`; `visible_memory_command.app_adapter.outcome` is registered in `apps/fomo/src/core/audit.ts`; targeted tests prove default-disabled no-write behavior, enabled sanitized structural audit-store writes, safe no-op statuses, user scoping, and private/cross-user leak exclusion; CI passed for head commit `2faee860`; PR merged as `8e16a709`; local main synced.

### PR-15: Disabled visible memory command app adapter audit-event seam

- **Exact branch:** `memory-v1-visible-memory-command-app-adapter-audit-disabled-seam`
- **Purpose:** Add the smallest safe disabled-by-default audit-event seam around `handleVisibleMemoryCommandAppAdapterRequest` so a future runtime caller can record sanitized visible-memory command outcomes through one stable audit envelope, without activating any live provider path, persisting raw private memory values/source refs, parsing arbitrary private text into memory, or changing current user-facing behavior.
- **Memory V1 Visible Behavior exit condition advanced:**
  1. remember explicit user preferences;
  2. retrieve relevant memories safely;
  3. explain why a memory was used;
  4. forget or correct a memory;
  5. prove source/audit metadata;
  6. prevent cross-user leakage;
  7. expose at least one visible memory behavior to the user.
- **Allowed files/areas:**
  - narrow disabled-by-default audit helper/type surface around `handleVisibleMemoryCommandAppAdapterRequest`;
  - tests proving the audit seam default is disabled and does not read/write/retract memory or write audit events;
  - tests proving enabled audit seam records only sanitized structural adapter outcome metadata;
  - tests proving unknown/non-memory text and missing parsed context remain no-ops with safe audit status only;
  - tests proving audit metadata excludes private raw values, source refs, arbitrary command text, and cross-user data;
  - tests proving user scoping and no cross-user leakage;
  - narrow queue advancement after PR-14.
- **Forbidden files/areas:**
  - production/runtime activation beyond a disabled test-level seam;
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
- **Expected changed files:** a small implementation/test slice in the existing FOMO memory-visible behavior area; no broad docs/harness changes except queue advancement after PR-14.
- **Tests required:** new disabled audit-event seam tests pass; existing visible memory remember/recall/review/explain/forget/correct/router/caller/unified caller/handler/app-adapter tests still pass; full lint/test/build and CI for exact PR commit.
- **Merge condition:** PR exists, CI green for exact PR commit, diff stays inside approved Memory V1 visible behavior scope, no forbidden surfaces touched.
- **Exit condition:** PR merged and local `main` synced; a future app/runtime caller can opt into one disabled-by-default audit seam for visible-memory command outcomes, recording only sanitized structural metadata without live activation or private/cross-user leakage.
- **Stop condition:** Stop and report owner/action if implementation requires migration/new table, production deploy, OAuth/security scope change, freeform LLM memory parsing, raw command text/private values in audit, or activation of Calendar/Composio/Tool Gateway/browser/action tools.
- **Founder approval needed?** No for a narrow disabled-by-default audit helper/test-level seam; yes before production deploy, new external scopes, irreversible data changes, freeform memory extraction/parsing, raw private audit persistence, or broad runtime activation.
- **Status:** Completed in PR #113, branch `memory-v1-visible-memory-command-app-adapter-audit-disabled-seam`, canonical commit `0b9faf2b9f6f1eb7b57030e34bfd77e37974f959`, merged as `3af23b09bfd03928e7cd7ed5b27a1b7e72327b09`.
- **Done condition met:** `VisibleMemoryCommandAppAdapterAuditEvent`, `VisibleMemoryCommandAppAdapterAuditEventRecorder`, and `VisibleMemoryCommandAppAdapterAuditOptions` exist in `apps/fomo/src/memory/typed-memory-visible-recall.ts`; `handleVisibleMemoryCommandAppAdapterRequest` accepts an optional disabled-by-default audit seam; targeted tests prove no audit event records when disabled/default, enabled audit records only sanitized structural adapter outcome metadata, unknown/non-memory text and missing parsed context remain safe no-ops, user scoping and no cross-user leakage are preserved, and private raw values/source refs/arbitrary command text do not leak; CI passed for head commit `0b9faf2b`; PR merged as `3af23b09`; local main synced.

### PR-14: Disabled visible memory command handler app-level adapter seam

- **Exact branch:** `memory-v1-visible-memory-command-handler-app-adapter-disabled-seam`
- **Purpose:** Add the smallest safe disabled-by-default app-level adapter seam around `handleVisibleMemoryCommandFromCaller` so a future runtime caller can pass explicit memory-command text and caller-supplied parsed remember/query/correction context through one stable request/response envelope, without activating any live provider path, parsing arbitrary private text into memory, or changing current user-facing behavior.
- **Memory V1 Visible Behavior exit condition advanced:**
  1. remember explicit user preferences;
  2. retrieve relevant memories safely;
  3. explain why a memory was used;
  4. forget or correct a memory;
  5. prove source/audit metadata;
  6. prevent cross-user leakage;
  7. expose at least one visible memory behavior to the user.
- **Allowed files/areas:**
  - narrow disabled-by-default adapter/type surface around `handleVisibleMemoryCommandFromCaller`;
  - tests proving the adapter default is disabled and does not read/write/retract memory;
  - tests proving enabled adapter calls the existing handler only with explicit caller-supplied parsed context;
  - tests proving unknown/non-memory text and missing parsed context remain no-ops;
  - tests proving adapter response/audit metadata exclude private raw values/source refs;
  - tests proving user scoping and no cross-user leakage;
  - narrow queue advancement after PR-13.
- **Forbidden files/areas:**
  - production/runtime activation beyond a disabled test-level seam;
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
- **Expected changed files:** a small implementation/test slice in the existing FOMO memory-visible behavior area; no broad docs/harness changes except queue advancement after PR-13.
- **Tests required:** new disabled app-level adapter seam tests pass; existing visible memory remember/recall/review/explain/forget/correct/router/caller/unified caller/handler tests still pass; full lint/test/build and CI for exact PR commit.
- **Merge condition:** PR exists, CI green for exact PR commit, diff stays inside approved Memory V1 visible behavior scope, no forbidden surfaces touched.
- **Exit condition:** PR merged and local `main` synced; a future app/runtime caller can use one disabled-by-default adapter seam for remember/review/explain/forget/correct only when explicitly enabled and supplied caller context, without live activation or private/cross-user leakage.
- **Stop condition:** Stop and report owner/action if implementation requires migration/new table, production deploy, OAuth/security scope change, freeform LLM memory parsing, or activation of Calendar/Composio/Tool Gateway/browser/action tools.
- **Founder approval needed?** No for a narrow disabled-by-default adapter/test-level seam; yes before production deploy, new external scopes, irreversible data changes, freeform memory extraction/parsing, or broad runtime activation.
- **Status:** Completed in PR #111, branch `memory-v1-visible-memory-command-handler-app-adapter-disabled-seam`, canonical commit `d5949ae072ffa162a513f3fa9c74d33cb4b2adc0`, merged as `70a009189564b84d26a94025bc7914b78387a571`.
- **Done condition met:** `handleVisibleMemoryCommandAppAdapterRequest`, `VisibleMemoryCommandAppAdapterRequest`, `VisibleMemoryCommandAppAdapterResult`, and `VisibleMemoryCommandAppAdapterStatus` exist in `apps/fomo/src/memory/typed-memory-visible-recall.ts`; targeted tests prove the app adapter is disabled by default without reads/writes/retractions; enabled adapter routes remember/review/explain/forget/correct through the existing handler only with explicit caller-supplied parsed context; unknown/non-memory text and missing parsed context are no-ops; user scoping and no cross-user leakage are preserved; inactive, stale, retracted, tombstoned, low-confidence, and cross-user memories remain excluded; private values and raw source refs do not leak; CI passed for head commit `d5949ae`; PR merged as `70a00918`; local main synced.

### PR-13: Disabled visible memory command handler seam

- **Exact branch:** `memory-v1-visible-memory-command-handler-disabled-seam`
- **Purpose:** Add the smallest safe disabled-by-default command-handler seam that can call `routeUnifiedVisibleMemoryCommandFromCaller` when an explicit caller enables it and supplies parsed remember/query/correction context, returning a stable user-facing response envelope plus audit-safe metadata without activating any live provider path or parsing arbitrary private text into memory.
- **Memory V1 Visible Behavior exit condition advanced:**
  1. remember explicit user preferences;
  2. retrieve relevant memories safely;
  3. explain why a memory was used;
  4. forget or correct a memory;
  5. prove source/audit metadata;
  6. prevent cross-user leakage;
  7. expose at least one visible memory behavior to the user.
- **Allowed files/areas:**
  - narrow disabled-by-default handler/type surface around `routeUnifiedVisibleMemoryCommandFromCaller`;
  - tests proving disabled/default handler is a no-op and does not write/retract/read live memory behavior unexpectedly;
  - tests proving enabled handler routes remember/review/explain/forget/correct only with explicit caller-supplied parsed context;
  - tests proving unknown/non-memory text and missing parsed context are no-ops;
  - tests proving response and audit metadata exclude private raw values/source refs;
  - tests proving user scoping and no cross-user leakage;
  - narrow queue advancement after PR-12.
- **Forbidden files/areas:**
  - production/runtime activation beyond a disabled test-level seam;
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
- **Expected changed files:** a small implementation/test slice in the existing FOMO memory-visible behavior area; no broad docs/harness changes except queue advancement after PR-12.
- **Tests required:** new disabled handler seam tests pass; existing visible memory remember/recall/review/explain/forget/correct/router/caller/unified caller tests still pass; full lint/test/build and CI for exact PR commit.
- **Merge condition:** PR exists, CI green for exact PR commit, diff stays inside approved Memory V1 visible behavior scope, no forbidden surfaces touched.
- **Exit condition:** PR merged and local `main` synced; an external caller can use one disabled-by-default handler seam for remember/review/explain/forget/correct only when explicitly enabled and supplied caller context, without live activation or private/cross-user leakage.
- **Stop condition:** Stop and report owner/action if implementation requires migration/new table, production deploy, OAuth/security scope change, freeform LLM memory parsing, or activation of Calendar/Composio/Tool Gateway/browser/action tools.
- **Founder approval needed?** No for a narrow disabled-by-default helper/test-level handler seam; yes before production deploy, new external scopes, irreversible data changes, freeform memory extraction/parsing, or broad runtime activation.
- **Status:** Completed in PR #109, branch `memory-v1-visible-memory-command-handler-disabled-seam`, canonical commit `e41f25f746524380512a192f6e853904d5da7c53`, merged as `3e70c9a18bc4b35aa584ab04a2db50b6bcf89446`.
- **Done condition met:** `handleVisibleMemoryCommandFromCaller`, `VisibleMemoryCommandHandlerContext`, `VisibleMemoryCommandHandlerResult`, and `VisibleMemoryCommandHandlerStatus` exist in `apps/fomo/src/memory/typed-memory-visible-recall.ts`; targeted tests prove the handler is disabled by default without reads/writes/retractions; enabled handler routes remember/review/explain/forget/correct only with explicit caller-supplied parsed context; unknown/non-memory text and missing parsed context are no-ops; user scoping and no cross-user leakage are preserved; inactive, stale, retracted, tombstoned, low-confidence, and cross-user memories remain excluded; private values and raw source refs do not leak; targeted test command passed on merge commit `3e70c9a1`; PR merged; local main synced.

### PR-12: Unified visible memory command caller seam

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
- **Status:** Completed in PR #108, branch `memory-v1-visible-memory-unified-caller-seam`, canonical commit `f165c0993fd2e69a654529c510d35e1bc8bd6d6e`, merged as `2368f382122c2a57a5f54797f37d5b1514d612fd`.
- **Done condition met:** `routeUnifiedVisibleMemoryCommandFromCaller`, `UnifiedVisibleMemoryCommandCallerContext`, and `UnifiedVisibleMemoryCommandCallerResult` exist in `apps/fomo/src/memory/typed-memory-visible-recall.ts`; targeted tests prove remember/review/explain/forget/correct route through one dormant caller seam only with explicit caller-supplied parsed context; unknown/non-memory text and missing parsed context are no-ops; user scoping and no cross-user leakage are preserved; inactive, stale, retracted, tombstoned, low-confidence, and cross-user memories remain excluded; private values and raw source refs do not leak; CI passed for head commit `f165c099`; PR merged; local main synced.

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

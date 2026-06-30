# Brevio Next PR Queue

Harness anchor loaded: BREVIO-HARNESS-V1-NO-CIRCLING-FAST-SHIPPING

There must be exactly one item marked `NEXT`. No cycle may say “continue M1” vaguely. Move the marker only after the current item is merged or explicitly blocked with owner/action.

## PR-B: M1 validation hardening

- **Purpose:** Add focused tests/validators for the current typed-memory facade and `memory_signals` bridge so M1-B safety is mechanically proven before retrieval-pack work.
- **Allowed files/areas:**
  - `apps/fomo/src/memory/typed-memory*.test.ts`
  - `apps/fomo/src/memory/typed-memory.ts` only for small validation helpers required by tests
  - `docs/M1_NO_MIGRATION_TYPED_MEMORY_FACADE.md` only for acceptance evidence if needed
- **Forbidden files/areas:**
  - migrations
  - schema changes/new tables
  - runtime caller wiring
  - ranker, HMR, reply-parser, Calendar, Composio, Tool Gateway, browser/action tools
  - unrelated docs or harness files
- **Expected changed files:** 1–3 files, preferably tests first.
- **Tests required:** targeted typed-memory test command(s), plus repo lint/test/build or CI for the PR commit.
- **Merge condition:** PR exists, CI green for the PR commit, diff stays inside allowed areas, no forbidden surfaces touched.
- **Done condition:** Added validation hardening proves cross-user isolation, deleted/tombstoned exclusion, metadata/null preservation, stable ordering, and no-migration boundary for the current facade/bridge.
- **Founder approval needed?** No, if it stays inside this contract.

## NEXT — PR-C: Typed retrieval pack builder, dormant only

- **Purpose:** Add retrieval-pack construction over existing `memory_signals` / typed facade without activating runtime consumers.
- **Allowed files/areas:** pure helper(s), tests, deterministic ordering, audit metadata.
- **Forbidden files/areas:** live ranking/HMR/reply-parser integration, migrations, new tables, production deploy.
- **Expected changed files:** 1–3 files under `apps/fomo/src/memory/`.
- **Tests required:** targeted retrieval-pack tests, privacy/cross-user tests, CI.
- **Merge condition:** CI green, dormant-only helper, no runtime imports from consumer surfaces.
- **Done condition:** A caller can construct a deterministic typed retrieval pack in tests only.
- **Founder approval needed?** No, if dormant only.

## PR-D: Memory audit/evidence helper surface, dormant only

- **Purpose:** Add evidence/debug helper that can explain which memory rows were considered/excluded.
- **Allowed files/areas:** helper functions, tests, no user-facing activation.
- **Forbidden files/areas:** UI/runtime exposure, HMR/reply-parser integration, migrations, new tables.
- **Expected changed files:** 1–3 files under `apps/fomo/src/memory/`.
- **Tests required:** targeted evidence helper tests, privacy/cross-user tests, CI.
- **Merge condition:** CI green, helper dormant, evidence contains ids/kinds/structural reasons only—not raw private content.
- **Done condition:** Tests can inspect considered/excluded memory rows safely.
- **Founder approval needed?** No, if dormant only.

## PR-E: M1-B closeout

- **Purpose:** Prove M1-B no-migration foundation is complete enough for the next phase and identify the next phase recommendation.
- **Allowed files/areas:** closeout doc, final validation checklist, queue update.
- **Forbidden files/areas:** reopening A/B/C, adding new architecture doctrine, runtime changes.
- **Expected changed files:** 1–2 docs.
- **Tests required:** cite exact merged PR CI runs and latest relevant validation tied to exact commits.
- **Merge condition:** CI green if docs CI runs; closeout names what is done/deferred/unlocked.
- **Done condition:** M1-B can be frozen and next phase queue starts from a concrete `NEXT` item.
- **Founder approval needed?** No for closeout; yes before starting next phase if it touches a real gate.

## Completed

### PR-A: Publish and merge docs/rulebook reconcile

- **Purpose:** Get local docs/rulebook reconcile out of local limbo through branch → PR → CI → merge flow.
- **Status:** Completed in PR #75, merged as `de54b57fa8d08896a12eac53099e061666a4c02b`; local `main` synced to `origin/main`.

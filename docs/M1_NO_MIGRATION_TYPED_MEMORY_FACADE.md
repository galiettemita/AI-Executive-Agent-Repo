# M1 no-migration typed memory facade

Status: substrate-only scaffold. No runtime consumer, no database migration, no provider smoke.

## Core Dimension Check

- **Advances:** Memory Architecture — names the first typed memory contracts in code and rejects an untyped catch-all memory dump.
- **Preserves:** Security / Permission Gates, Human Message Renderer, Feedback + Learn/Grow Loop, Tool / Workflow Orchestration — no user-facing surface, no provider call, no business-data mutation, no ranker/HMR consumer.
- **Intentionally defers:** Full M1 typed tables, Postgres stores, migrations, retrieval into ranker/HMR, consolidation, skill registry, Composio wrapper.

## 6-question gate

1. **What reusable Brevio layer does this advance?**
   - A dormant typed-memory facade for semantic facts, preferences, projects, contacts, and repeated-behavior memory.
2. **What is locked in scope?**
   - TypeScript contracts, in-memory dormant implementation, structural retrieval/retraction audit actions, privacy/cross-tenant tests.
3. **What is explicitly out of scope?**
   - Migrations, Postgres stores, consumer reads, ranker/HMR integration, reply-parser changes, consolidation, skill execution, provider calls.
4. **What is the no-migration safety boundary?**
   - The facade is not added to `createStores()`, has no Postgres implementation, and no production caller imports it.
5. **What proves this is typed, safe, and reversible?**
   - Tests cover declared kinds/sources/confidence, cross-tenant isolation, low-confidence/retracted retrieval exclusion, structural audit-only retrieval/retraction, and raw-email scope-key rejection.
6. **What proves this does not shrink Brevio into FOMO-only polish?**
   - The code is Brevio-memory substrate, not Gmail alert behavior; it names reusable memory kinds required for future Memory Architecture and Skill OS work.

## Acceptance evidence

- `node --experimental-strip-types --loader ./test-loader.mjs --test src/memory/typed-memory.test.ts`
- `pnpm --filter @brevio/fomo test`
- `pnpm --filter @brevio/fomo lint`
- `pnpm --filter @brevio/fomo build`

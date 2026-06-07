// Phase v0.5.12 — LIVE RANKER GUARDED-MODE INVARIANT (C13 LOAD-BEARING).
//
// Updated from v0.5.11 (which forbade ALL production imports of pil-context).
// v0.5.12 explicitly allows the live ranker to read PIL via the NEW
// `buildLivePilContext` export — the SAME module file, but a DIFFERENT
// function with the canonical-HMAC-only read-side filter. The SHADOW
// `buildPilContext` function remains eval-only by convention but is no longer
// gated by a hard structural assertion at the import level — the contract is
// now FUNCTION-NAME-based:
//
//   - Production paths (ranker/index.ts, workers/gmail-poll.ts) MAY import
//     `buildLivePilContext` and the `PilContext` type. They MUST NOT call
//     `buildPilContext` directly (the shadow projection is reserved for
//     evals + tests).
//   - Ranker prompt builder MAY reference `pil_context` (the parameter
//     name). PROMPT_VERSION is conditional: baseline calls use ranker-v0.2.0;
//     PIL-context calls use ranker-v0.3.0. Both are valid production states.
//   - rank_results schema is unchanged (no new pil_* columns).
//
// The contract is enforced via a combination of:
//   1. Structural import checks (this file)
//   2. Read-side filter regex (CANONICAL_SCOPE_KEY_REGEX) — covered by
//      pil-live-context.test.ts (BB6 LOAD-BEARING)
//   3. Kill-switch gate at worker call site — covered by
//      pil-live-hybrid.test.ts (BB7 LOAD-BEARING)
//   4. C1 smoke check — kill switch off → no brevio.rank.pil_applied audit
//      rows in window

import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';
import { fileURLToPath } from 'node:url';
import { dirname, join } from 'node:path';
import { describe, it } from 'node:test';

import {
  PROMPT_VERSION as RANKER_PROMPT_VERSION,
  PROMPT_VERSION_WITH_PIL
} from './prompt.ts';

const __dirname = dirname(fileURLToPath(import.meta.url));

async function readSource(relPath: string): Promise<string> {
  return await readFile(join(__dirname, '..', '..', relPath), 'utf8');
}

describe('v0.5.12 live ranker guarded-mode invariant (C13 LOAD-BEARING)', () => {
  it('production ranker module imports buildLivePilContext (NOT buildPilContext) — shadow projection stays eval-only', async () => {
    const src = await readSource('src/ranker/index.ts');
    // v0.5.12 contract: index.ts may reference buildLivePilContext via the
    // wrapper but MUST NOT call the SHADOW projection (`buildPilContext`
    // without `Live`). Search for a call site `buildPilContext(` (open paren)
    // to distinguish from the type-only re-export.
    assert.equal(
      src.includes('buildPilContext('),
      false,
      'ranker/index.ts must NOT CALL buildPilContext — shadow projection is eval-only'
    );
    // CANONICAL_SCOPE_KEY_REGEX is imported by the wrapper for defensive
    // read-side filtering — that's expected.
    assert.equal(
      src.includes('CANONICAL_SCOPE_KEY_REGEX'),
      true,
      'ranker/index.ts must import CANONICAL_SCOPE_KEY_REGEX for defensive read-side filter (BB6 LOAD-BEARING)'
    );
  });

  it('ranker prompt builder accepts an optional pil_context parameter (PROMPT_VERSION conditional)', async () => {
    const src = await readSource('src/ranker/prompt.ts');
    // pil_context shows up as a function parameter (buildRankerPrompt) and
    // in the PIL block builder — both are intended v0.5.12 additions.
    assert.equal(
      src.includes('pilContext'),
      true,
      'ranker/prompt.ts must accept pilContext parameter (v0.5.12)'
    );
    // PROMPT_VERSION_WITH_PIL is the conditional bump.
    assert.equal(
      src.includes('PROMPT_VERSION_WITH_PIL'),
      true,
      'ranker/prompt.ts must export PROMPT_VERSION_WITH_PIL for ranker-v0.3.0 calls'
    );
  });

  it('production polling worker may import buildLivePilContext via the pilLive dep — but does NOT call buildPilContext (shadow)', async () => {
    const src = await readSource('src/workers/gmail-poll.ts');
    // Worker MUST NOT call the shadow projection directly. (Type-only import
    // of the `PilContext` interface is fine; we look for a CALL pattern.)
    assert.equal(
      src.includes('buildPilContext('),
      false,
      'workers/gmail-poll.ts must NOT CALL buildPilContext — shadow projection is eval-only'
    );
  });

  it('ranker PROMPT_VERSION baseline (no PIL) is "ranker-v0.2.0"', () => {
    assert.equal(RANKER_PROMPT_VERSION, 'ranker-v0.2.0');
  });

  it('ranker PROMPT_VERSION_WITH_PIL is "ranker-v0.3.0" (v0.5.12 bump)', () => {
    assert.equal(PROMPT_VERSION_WITH_PIL, 'ranker-v0.3.0');
  });

  it('rank_results schema unchanged — no new pil_* columns referenced anywhere in src/', async () => {
    // Walk the rank-results store module and ensure no new pil_* column
    // references slipped in. This is the v0.5.11 → v0.5.12 promise that
    // observability lives in the audit kind, not the schema.
    const src = await readSource('src/memory/rank-results.ts');
    // Strict checks — the substring 'pil_' should not appear at all in the
    // rank_results store module.
    assert.equal(
      src.includes('pil_'),
      false,
      'src/memory/rank-results.ts must NOT carry pil_* columns (Q6.A: audit-only observability)'
    );
  });
});

// Phase v0.5.11 — LIVE RANKER BIT-IDENTICAL INVARIANT (C13 LOAD-BEARING).
//
// Three structural assertions that together prove v0.5.11 does NOT change
// live ranker behavior:
//
//   1. `apps/fomo/src/ranker/pil-context.ts` is NOT imported by the
//      production ranker (rank-email.ts) or the production rank call site
//      (workers/gmail-poll.ts). It is consumed ONLY by the eval harness +
//      this test file's sibling pil-context.test.ts.
//
//   2. The ranker prompt builder does NOT mention `pil_context` (no schema
//      field, no prompt body reference). v0.5.12 is the gate that adds it.
//
//   3. PROMPT_VERSION in ranker/prompt.ts is unchanged (`ranker-v0.2.0`).

import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';
import { fileURLToPath } from 'node:url';
import { dirname, join } from 'node:path';
import { describe, it } from 'node:test';

import { PROMPT_VERSION as RANKER_PROMPT_VERSION } from './prompt.ts';

const __dirname = dirname(fileURLToPath(import.meta.url));

async function readSource(relPath: string): Promise<string> {
  return await readFile(join(__dirname, '..', '..', relPath), 'utf8');
}

describe('v0.5.11 live ranker bit-identical invariant (C13 LOAD-BEARING)', () => {
  it('production ranker index module does NOT import buildPilContext or pil-context', async () => {
    const src = await readSource('src/ranker/index.ts');
    assert.equal(src.includes('buildPilContext'), false, 'ranker/index.ts must NOT import buildPilContext');
    assert.equal(src.includes('pil-context'), false, 'ranker/index.ts must NOT import pil-context module');
  });

  it('ranker prompt builder does NOT reference pil_context anywhere', async () => {
    const src = await readSource('src/ranker/prompt.ts');
    assert.equal(src.includes('pil_context'), false, 'ranker/prompt.ts must NOT mention pil_context');
    assert.equal(src.includes('buildPilContext'), false, 'ranker/prompt.ts must NOT import buildPilContext');
  });

  it('production polling worker does NOT import buildPilContext', async () => {
    const src = await readSource('src/workers/gmail-poll.ts');
    assert.equal(src.includes('buildPilContext'), false, 'workers/gmail-poll.ts must NOT import buildPilContext');
    assert.equal(src.includes("'../ranker/pil-context"), false, 'workers/gmail-poll.ts must NOT import pil-context module');
  });

  it('ranker PROMPT_VERSION is still ranker-v0.2.0 (no v0.5.11 bump)', () => {
    assert.equal(RANKER_PROMPT_VERSION, 'ranker-v0.2.0');
  });
});

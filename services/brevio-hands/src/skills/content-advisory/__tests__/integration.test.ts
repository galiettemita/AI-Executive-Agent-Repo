import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { dirname, join } from 'node:path';
import { describe, it } from 'node:test';
import { fileURLToPath } from 'node:url';

import adapter from '../index.js';

interface Fixture {
  status: 'SUCCESS';
  provider: string;
  state: string;
  input: Record<string, unknown>;
}

const testDir = dirname(fileURLToPath(import.meta.url));
const readFixture = (): Fixture =>
  JSON.parse(readFileSync(join(testDir, 'fixtures', 'success.json'), 'utf8')) as Fixture;

describe('content-advisory integration', () => {
  it('returns deterministic fixture-backed output', async () => {
    const expected = readFixture();
    const result = await adapter.execute(expected.input, {} as never);

    assert.equal(result.status, expected.status);
    if (expected.provider.length > 0) {
      assert.equal((result.data as { provider?: string } | undefined)?.provider, expected.provider);
    }
    if (expected.state.length > 0) {
      assert.equal((result.data as { state?: string } | undefined)?.state, expected.state);
    }
  });
});

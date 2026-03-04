import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { dirname, join } from 'node:path';
import { describe, it } from 'node:test';
import { fileURLToPath } from 'node:url';

import adapter from '../index.js';

interface Fixture {
  preview_id: string;
  summary: string;
}

const testDir = dirname(fileURLToPath(import.meta.url));

function readFixture(): Fixture {
  return JSON.parse(readFileSync(join(testDir, 'fixtures', 'render-success.json'), 'utf8')) as Fixture;
}

describe('react-email-skills integration', () => {
  it('returns deterministic template rendering output', async () => {
    const result = await adapter.execute(
      { action: 'render_template', template_id: 'weekly-update', subject: 'Weekly Update' },
      {} as never
    );

    const expected = readFixture();
    assert.equal(result.status, 'SUCCESS');
    assert.equal(result.data?.preview_id, expected.preview_id);
    assert.equal(result.data?.summary, expected.summary);
    assert.match(result.data?.html ?? '', /<h1>Weekly Update<\/h1>/);
  });
});

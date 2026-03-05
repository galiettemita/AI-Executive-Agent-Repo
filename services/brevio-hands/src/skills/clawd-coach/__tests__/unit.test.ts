import assert from 'node:assert/strict';
import { describe, it } from 'node:test';

import adapter from '../index.js';

describe('clawd-coach adapter', () => {
  it('requires goal for build_plan', async () => {
    const result = await adapter.execute({ action: 'build_plan' }, {} as never);
    assert.equal(result.status, 'FAILED');
    assert.match(result.error?.message ?? '', /CLAWD_COACH_GOAL_REQUIRED/);
  });

  it('returns training plan payload', async () => {
    const result = await adapter.execute({ action: 'build_plan', goal: 'Half marathon', weeks: 12 }, {} as never);
    assert.equal(result.status, 'SUCCESS');
    assert.equal(result.data?.provider, 'clawd-coach');
  });
});

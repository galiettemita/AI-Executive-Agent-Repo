import assert from 'node:assert/strict';
import { describe, it } from 'node:test';

import adapter from '../index.js';

describe('sports-ticker adapter', () => {
  it('requires league and team for score lookup', async () => {
    const result = await adapter.execute({ action: 'get_score', league: 'nba' }, {} as never);
    assert.equal(result.status, 'FAILED');
    assert.match(result.error?.message ?? '', /SPORTS_TICKER_TEAM_REQUIRED/);
  });

  it('returns schedule payload', async () => {
    const result = await adapter.execute({ action: 'get_schedule', league: 'nba' }, {} as never);
    assert.equal(result.status, 'SUCCESS');
    assert.equal(result.data?.provider, 'sports-ticker');
  });
});

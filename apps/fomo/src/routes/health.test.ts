import assert from 'node:assert/strict';
import { describe, it } from 'node:test';

import { buildHealthResponse } from './health.ts';

describe('buildHealthResponse', () => {
  it('returns status ok and computed uptime', () => {
    const config = {
      serviceName: 'fomo',
      version: '0.1.0',
      environment: 'test',
      port: 8080,
      shutdownTimeoutMs: 30000
    };
    const started = 1_000;
    const now = 5_500;
    const r = buildHealthResponse(config, started, now);
    assert.equal(r.status, 'ok');
    assert.equal(r.service, 'fomo');
    assert.equal(r.version, '0.1.0');
    assert.equal(r.uptime_ms, 4_500);
  });
});

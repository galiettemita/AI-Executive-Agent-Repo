import assert from 'node:assert/strict';
import { describe, it } from 'node:test';

import { RateLimiter, configureDefaults } from './rate-limit.ts';

describe('RateLimiter', () => {
  it('returns allowed when no policy configured', () => {
    const limiter = new RateLimiter();
    const result = limiter.consume('GET /unknown', 'user-1');
    assert.equal(result.allowed, true);
  });

  it('exhausts capacity then blocks', () => {
    const limiter = new RateLimiter();
    limiter.configure('POST /test', { capacity: 3, refillPerMs: 0 });
    let now = 1000;
    assert.equal(limiter.consume('POST /test', 'user-1', now).allowed, true);
    assert.equal(limiter.consume('POST /test', 'user-1', now).allowed, true);
    assert.equal(limiter.consume('POST /test', 'user-1', now).allowed, true);
    const blocked = limiter.consume('POST /test', 'user-1', now);
    assert.equal(blocked.allowed, false);
    assert.ok(blocked.retryAfterMs >= 0);
  });

  it('refills tokens over time', () => {
    const limiter = new RateLimiter();
    limiter.configure('POST /test', { capacity: 1, refillPerMs: 0.001 }); // 1 token / 1000ms
    let now = 1000;
    assert.equal(limiter.consume('POST /test', 'u', now).allowed, true);
    assert.equal(limiter.consume('POST /test', 'u', now).allowed, false);
    now += 1500;
    assert.equal(limiter.consume('POST /test', 'u', now).allowed, true);
  });

  it('isolates buckets per key', () => {
    const limiter = new RateLimiter();
    limiter.configure('POST /test', { capacity: 1, refillPerMs: 0 });
    let now = 1000;
    assert.equal(limiter.consume('POST /test', 'a', now).allowed, true);
    assert.equal(limiter.consume('POST /test', 'a', now).allowed, false);
    assert.equal(limiter.consume('POST /test', 'b', now).allowed, true);
  });

  it('isolates buckets per endpoint', () => {
    const limiter = new RateLimiter();
    limiter.configure('POST /a', { capacity: 1, refillPerMs: 0 });
    limiter.configure('POST /b', { capacity: 1, refillPerMs: 0 });
    const now = 1000;
    assert.equal(limiter.consume('POST /a', 'u', now).allowed, true);
    assert.equal(limiter.consume('POST /a', 'u', now).allowed, false);
    assert.equal(limiter.consume('POST /b', 'u', now).allowed, true);
  });
});

describe('configureDefaults', () => {
  it('configures all expected endpoints', () => {
    const limiter = new RateLimiter();
    configureDefaults(limiter);
    const consent = limiter.consume('POST /api/v1/me/consent', 'u');
    assert.equal(consent.allowed, true);
    const skills = limiter.consume('GET /api/v1/me/skills/enabled', 'u');
    assert.equal(skills.allowed, true);
  });
});

// Token-bucket rate limiter, per (key, endpoint). In-memory; single-process only.
// Multi-instance Node gateway should swap to Redis-backed limiter (deferred).

export interface RateLimitConfig {
  capacity: number;       // max tokens
  refillPerMs: number;    // tokens added per millisecond
}

export interface RateLimitDecision {
  allowed: boolean;
  remaining: number;
  retryAfterMs: number;
}

interface BucketState {
  tokens: number;
  lastRefillMs: number;
}

export class RateLimiter {
  private readonly buckets = new Map<string, BucketState>();
  private readonly configs = new Map<string, RateLimitConfig>();

  configure(endpoint: string, config: RateLimitConfig): void {
    this.configs.set(endpoint, config);
  }

  consume(endpoint: string, key: string, now: number = Date.now()): RateLimitDecision {
    const config = this.configs.get(endpoint);
    if (!config) {
      return { allowed: true, remaining: Number.POSITIVE_INFINITY, retryAfterMs: 0 };
    }
    const compositeKey = `${endpoint}::${key}`;
    let bucket = this.buckets.get(compositeKey);
    if (!bucket) {
      bucket = { tokens: config.capacity, lastRefillMs: now };
      this.buckets.set(compositeKey, bucket);
    } else {
      const elapsed = Math.max(0, now - bucket.lastRefillMs);
      const refill = elapsed * config.refillPerMs;
      bucket.tokens = Math.min(config.capacity, bucket.tokens + refill);
      bucket.lastRefillMs = now;
    }

    if (bucket.tokens >= 1) {
      bucket.tokens -= 1;
      return { allowed: true, remaining: Math.floor(bucket.tokens), retryAfterMs: 0 };
    }
    const deficit = 1 - bucket.tokens;
    const retryAfterMs = Math.ceil(deficit / config.refillPerMs);
    return { allowed: false, remaining: 0, retryAfterMs };
  }

  reset(): void {
    this.buckets.clear();
  }
}

// Default policies per the plan §2.5. Refill rates derived from window/refills:
//   30 / 5min refill = 6/min = 0.0001 tokens/ms
//   10 / 5min refill = 2/min = 0.0000333 tokens/ms
//   20 / hour       = 20/3600s = 0.00000555 tokens/ms
//   50 / hour       = 50/3600s = 0.0000139 tokens/ms
//   600 / min       = 10/sec = 0.01 tokens/ms

export function configureDefaults(limiter: RateLimiter): void {
  limiter.configure('POST /api/v1/me/consent', { capacity: 30, refillPerMs: 0.0001 });
  limiter.configure('POST /api/v1/me/consent/revoke', { capacity: 10, refillPerMs: 1 / 30000 });
  limiter.configure('GET /api/v1/oauth/start', { capacity: 20, refillPerMs: 20 / 3600000 });
  limiter.configure('GET /api/v1/oauth/callback', { capacity: 50, refillPerMs: 50 / 3600000 });
  limiter.configure('GET /api/v1/me/skills/enabled', { capacity: 600, refillPerMs: 0.01 });
}

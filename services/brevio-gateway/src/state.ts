import { randomUUID } from 'node:crypto';

import type { Channel, DedupCachedResponse, GatewayConfig, RateLimitDecision, SessionState, UserTier } from './types.js';

interface PrunedWindows {
  hourWindow: number[];
  minuteWindow: number[];
}

export class GatewayState {
  private readonly dedupCache = new Map<string, DedupCachedResponse>();
  private readonly userWindows = new Map<string, number[]>();
  private readonly sessions = new Map<string, SessionState>();

  prune(nowMs: number): void {
    for (const [key, value] of this.dedupCache.entries()) {
      if (value.expiresAtMs <= nowMs) {
        this.dedupCache.delete(key);
      }
    }
  }

  getCachedResponse(dedupKey: string, nowMs: number): DedupCachedResponse | null {
    const cached = this.dedupCache.get(dedupKey);
    if (!cached) {
      return null;
    }
    if (cached.expiresAtMs <= nowMs) {
      this.dedupCache.delete(dedupKey);
      return null;
    }
    return cached;
  }

  cacheResponse(dedupKey: string, statusCode: number, payload: Record<string, unknown>, nowMs: number, ttlMs: number): void {
    this.dedupCache.set(dedupKey, {
      statusCode,
      payload,
      expiresAtMs: nowMs + ttlMs
    });
  }

  sessionForUser(channel: Channel, userId: string, providedSessionId: string | undefined, nowMs: number, idleMs: number): string {
    const sessionKey = `${channel}:${userId}`;
    if (providedSessionId && providedSessionId.trim() !== '') {
      this.sessions.set(sessionKey, {
        sessionId: providedSessionId,
        lastActivityMs: nowMs
      });
      return providedSessionId;
    }

    const existing = this.sessions.get(sessionKey);
    if (!existing) {
      const newSession = randomUUID();
      this.sessions.set(sessionKey, {
        sessionId: newSession,
        lastActivityMs: nowMs
      });
      return newSession;
    }

    if (nowMs-existing.lastActivityMs > idleMs) {
      const rotated = randomUUID();
      this.sessions.set(sessionKey, {
        sessionId: rotated,
        lastActivityMs: nowMs
      });
      return rotated;
    }

    existing.lastActivityMs = nowMs;
    return existing.sessionId;
  }

  checkRateLimit(userId: string, tier: UserTier, nowMs: number, config: GatewayConfig): RateLimitDecision {
    const windows = this.userWindows.get(userId) ?? [];
    const pruned = this.pruneWindows(windows, nowMs, config.rateLimitWindowMs, config.rateLimitMinuteWindowMs);

    if (pruned.minuteWindow.length >= config.rateLimitPerMinute) {
      const retryAfterSeconds = Math.max(1, Math.ceil((pruned.minuteWindow[0] + config.rateLimitMinuteWindowMs - nowMs) / 1000));
      return {
        allowed: false,
        reason: 'RATE_LIMIT_MINUTE',
        limit: config.rateLimitPerMinute,
        remaining: 0,
        retryAfterSeconds
      };
    }

    const hourLimit = this.hourLimitForTier(tier, config);
    if (Number.isFinite(hourLimit) && pruned.hourWindow.length >= hourLimit) {
      const retryAfterSeconds = Math.max(1, Math.ceil((pruned.hourWindow[0] + config.rateLimitWindowMs - nowMs) / 1000));
      return {
        allowed: false,
        reason: 'RATE_LIMIT_HOUR',
        limit: hourLimit,
        remaining: 0,
        retryAfterSeconds
      };
    }

    pruned.hourWindow.push(nowMs);
    this.userWindows.set(userId, pruned.hourWindow);

    const remaining = Number.isFinite(hourLimit)
      ? Math.max(0, hourLimit - pruned.hourWindow.length)
      : Number.MAX_SAFE_INTEGER;

    return {
      allowed: true,
      limit: Number.isFinite(hourLimit) ? hourLimit : Number.MAX_SAFE_INTEGER,
      remaining,
      retryAfterSeconds: 0
    };
  }

  stats(): { dedupEntries: number; trackedUsers: number; activeSessions: number } {
    return {
      dedupEntries: this.dedupCache.size,
      trackedUsers: this.userWindows.size,
      activeSessions: this.sessions.size
    };
  }

  private pruneWindows(entries: number[], nowMs: number, hourWindowMs: number, minuteWindowMs: number): PrunedWindows {
    const hourWindow = entries.filter((timestamp) => nowMs - timestamp < hourWindowMs);
    const minuteWindow = hourWindow.filter((timestamp) => nowMs - timestamp < minuteWindowMs);

    return {
      hourWindow,
      minuteWindow
    };
  }

  private hourLimitForTier(tier: UserTier, config: GatewayConfig): number {
    switch (tier) {
      case 'free':
        return config.rateLimitFreePerHour;
      case 'pro':
        return config.rateLimitProPerHour;
      case 'enterprise':
      case 'admin':
      case 'service':
        return Number.POSITIVE_INFINITY;
      default:
        return config.rateLimitFreePerHour;
    }
  }
}

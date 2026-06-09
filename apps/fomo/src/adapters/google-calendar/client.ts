// Phase v0.6.0C — Google Calendar HTTP client (read-only).
//
// Mirrors the GmailClient shape ([apps/fomo/src/adapters/gmail/client.ts]):
// direct fetch, no SDK, injectable FetchLike, caller-supplied access_token,
// stable typed projection at the boundary.
//
// Scope: `https://www.googleapis.com/auth/calendar.events.readonly`.
// Primary calendar only — the calendarId path segment is literally
// 'primary' (per [docs/v0.6.0B-oauth-scope-readiness.md §1] decision row 2).
// No write methods. Adding a write method here is a deliberate code
// change with founder review.

import { type RawGoogleCalendarEvent } from './types.js';

export type FetchLike = typeof fetch;

/**
 * Read-only Calendar scope. Hardcoded — v0.6.0C requests no other
 * Calendar scope and `calendar.readonly` (broader) is explicitly
 * rejected per [docs/v0.6.0B-oauth-scope-readiness.md §2.2].
 */
export const CALENDAR_EVENTS_READONLY_SCOPE =
  'https://www.googleapis.com/auth/calendar.events.readonly';

const CALENDAR_API_BASE = 'https://www.googleapis.com/calendar/v3';

export class CalendarUnauthorizedError extends Error {
  readonly httpStatus: 401;
  constructor(reason: string) {
    super(`Calendar returned 401: ${reason}`);
    this.name = 'CalendarUnauthorizedError';
    this.httpStatus = 401;
  }
}

export class CalendarApiError extends Error {
  readonly httpStatus: number;
  readonly providerCode: string | undefined;
  readonly retryable: boolean;
  constructor(httpStatus: number, providerCode: string | undefined, reason: string) {
    super(
      `Calendar API error (${httpStatus}${providerCode ? ` ${providerCode}` : ''}): ${reason}`
    );
    this.name = 'CalendarApiError';
    this.httpStatus = httpStatus;
    this.providerCode = providerCode;
    this.retryable = httpStatus >= 500 || httpStatus === 429;
  }
}

export interface GoogleCalendarClientConfig {
  readonly fetchImpl?: FetchLike;
  readonly timeoutMs?: number;
}

export interface ListPrimaryEventsOpts {
  /** ISO 8601 lower bound (inclusive). */
  readonly timeMin: string;
  /** ISO 8601 upper bound (exclusive). */
  readonly timeMax: string;
  /**
   * Maximum events to fetch. Per Google's API documentation this is
   * capped at 2500 server-side; we cap at 250 client-side because v0.6.0C
   * does not need more, and a larger response would just be discarded.
   */
  readonly maxResults?: number;
}

export class GoogleCalendarClient {
  private readonly fetchImpl: FetchLike;
  private readonly timeoutMs: number;

  constructor(config: GoogleCalendarClientConfig = {}) {
    this.fetchImpl = config.fetchImpl ?? fetch;
    this.timeoutMs = config.timeoutMs ?? 30_000;
  }

  /**
   * List events on the user's PRIMARY calendar within [timeMin, timeMax).
   *
   * Returns the raw events array (each entry typed as
   * `RawGoogleCalendarEvent`). The CALLER is responsible for projecting
   * each raw entry through `projectCalendarEvent` at the adapter boundary
   * — this method intentionally does NOT project, so the boundary is
   * one and the same place where the field-exclusion contract is tested
   * (see context-source.test.ts).
   *
   * `singleEvents=true` is forced so recurring events expand into single
   * instances (eliminates the recurringEventId field path entirely — the
   * non-instance recurring "parent" never reaches us).
   *
   * `orderBy=startTime` is forced so the nearest-event calculation is
   * deterministic without a second sort.
   */
  async listPrimaryEvents(
    accessToken: string,
    opts: ListPrimaryEventsOpts
  ): Promise<readonly RawGoogleCalendarEvent[]> {
    const params = new URLSearchParams({
      timeMin: opts.timeMin,
      timeMax: opts.timeMax,
      singleEvents: 'true',
      orderBy: 'startTime',
      maxResults: String(Math.min(opts.maxResults ?? 50, 250))
    });
    const url = `${CALENDAR_API_BASE}/calendars/primary/events?${params.toString()}`;
    const json = await this.get(url, accessToken);
    const root = json as Record<string, unknown>;
    const items = root.items;
    if (!Array.isArray(items)) {
      return Object.freeze([]);
    }
    // Defensive copy: freeze the array so callers cannot mutate, but do
    // NOT recursively freeze items — the projection function reads only
    // the four whitelisted keys and discards the rest.
    return Object.freeze(items as readonly RawGoogleCalendarEvent[]);
  }

  private async get(url: string, accessToken: string): Promise<unknown> {
    const controller = new AbortController();
    const timer = setTimeout(() => controller.abort(), this.timeoutMs);
    let res: Response;
    try {
      res = await this.fetchImpl(url, {
        method: 'GET',
        headers: {
          authorization: `Bearer ${accessToken}`,
          accept: 'application/json'
        },
        signal: controller.signal
      });
    } catch (err) {
      clearTimeout(timer);
      throw new CalendarApiError(0, undefined, err instanceof Error ? err.message : String(err));
    }
    clearTimeout(timer);
    if (res.status === 401) {
      throw new CalendarUnauthorizedError('access token rejected');
    }
    let body: unknown;
    try {
      body = await res.json();
    } catch {
      if (!res.ok) {
        throw new CalendarApiError(res.status, undefined, 'non-JSON response');
      }
      return {};
    }
    if (!res.ok) {
      const errObj = (body as { error?: { code?: number; message?: string; status?: string } })
        .error;
      throw new CalendarApiError(
        res.status,
        errObj?.status,
        errObj?.message ?? `HTTP ${res.status}`
      );
    }
    return body;
  }
}

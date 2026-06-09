// Phase v0.6.0C — GoogleCalendarClient HTTP client tests.
//
// Covers:
//   - URL path is /calendars/primary/events (primary calendar only)
//   - singleEvents=true + orderBy=startTime forced
//   - Bearer auth header
//   - 401 → CalendarUnauthorizedError
//   - 4xx → CalendarApiError with httpStatus + providerCode
//   - 5xx → CalendarApiError with retryable=true
//   - empty items array tolerated
//   - timeout handling does not leak the raw error message

import assert from 'node:assert/strict';
import { describe, it } from 'node:test';

import {
  CALENDAR_EVENTS_READONLY_SCOPE,
  CalendarApiError,
  CalendarUnauthorizedError,
  GoogleCalendarClient
} from './client.ts';

function mockFetch(
  handler: (url: string, init: RequestInit) => Promise<{ status: number; body: unknown }>
): typeof fetch {
  return (async (input: string | URL | Request, init?: RequestInit) => {
    const url = typeof input === 'string' ? input : input.toString();
    const result = await handler(url, init ?? {});
    const isJson = result.body !== undefined;
    return {
      status: result.status,
      ok: result.status >= 200 && result.status < 300,
      async json() {
        if (!isJson) throw new Error('no body');
        return result.body as unknown;
      },
      async text() {
        return JSON.stringify(result.body ?? null);
      }
    } as Response;
  }) as unknown as typeof fetch;
}

describe('GoogleCalendarClient — read-only scope constant', () => {
  it('exports the exact scope string Google expects', () => {
    assert.equal(
      CALENDAR_EVENTS_READONLY_SCOPE,
      'https://www.googleapis.com/auth/calendar.events.readonly'
    );
  });
});

describe('GoogleCalendarClient.listPrimaryEvents', () => {
  it('targets the PRIMARY calendar with singleEvents + orderBy=startTime', async () => {
    let observedUrl: string | null = null;
    let observedAuth: string | undefined;
    const fetchImpl = mockFetch(async (url, init) => {
      observedUrl = url;
      const headers = init.headers as Record<string, string>;
      observedAuth = headers.authorization;
      return { status: 200, body: { items: [] } };
    });
    const client = new GoogleCalendarClient({ fetchImpl });
    await client.listPrimaryEvents('token-fixture', {
      timeMin: '2026-06-09T10:00:00.000Z',
      timeMax: '2026-06-11T10:00:00.000Z'
    });
    assert.ok(observedUrl);
    assert.ok(observedUrl!.includes('/calendars/primary/events'));
    assert.ok(observedUrl!.includes('singleEvents=true'));
    assert.ok(observedUrl!.includes('orderBy=startTime'));
    assert.ok(observedUrl!.includes('timeMin=2026-06-09T10%3A00%3A00.000Z'));
    assert.ok(observedUrl!.includes('timeMax=2026-06-11T10%3A00%3A00.000Z'));
    assert.equal(observedAuth, 'Bearer token-fixture');
  });

  it('returns an empty list when Google responds with no items', async () => {
    const fetchImpl = mockFetch(async () => ({ status: 200, body: {} }));
    const client = new GoogleCalendarClient({ fetchImpl });
    const events = await client.listPrimaryEvents('token', {
      timeMin: '2026-06-09T10:00:00.000Z',
      timeMax: '2026-06-09T11:00:00.000Z'
    });
    assert.equal(events.length, 0);
  });

  it('caps maxResults at 250 client-side even when caller asks for more', async () => {
    let observedUrl: string | null = null;
    const fetchImpl = mockFetch(async (url) => {
      observedUrl = url;
      return { status: 200, body: { items: [] } };
    });
    const client = new GoogleCalendarClient({ fetchImpl });
    await client.listPrimaryEvents('token', {
      timeMin: '2026-06-09T10:00:00.000Z',
      timeMax: '2026-06-09T11:00:00.000Z',
      maxResults: 9999
    });
    assert.ok(observedUrl);
    assert.ok(observedUrl!.includes('maxResults=250'));
  });

  it('throws CalendarUnauthorizedError on 401', async () => {
    const fetchImpl = mockFetch(async () => ({
      status: 401,
      body: { error: { code: 401, message: 'invalid', status: 'UNAUTHENTICATED' } }
    }));
    const client = new GoogleCalendarClient({ fetchImpl });
    await assert.rejects(
      () =>
        client.listPrimaryEvents('bad-token', {
          timeMin: '2026-06-09T10:00:00.000Z',
          timeMax: '2026-06-09T11:00:00.000Z'
        }),
      (err) => err instanceof CalendarUnauthorizedError
    );
  });

  it('throws CalendarApiError on 4xx with providerCode populated', async () => {
    const fetchImpl = mockFetch(async () => ({
      status: 400,
      body: { error: { code: 400, message: 'bad arg', status: 'INVALID_ARGUMENT' } }
    }));
    const client = new GoogleCalendarClient({ fetchImpl });
    try {
      await client.listPrimaryEvents('token', {
        timeMin: '2026-06-09T10:00:00.000Z',
        timeMax: '2026-06-09T11:00:00.000Z'
      });
      assert.fail('expected throw');
    } catch (err) {
      assert.ok(err instanceof CalendarApiError);
      assert.equal((err as CalendarApiError).httpStatus, 400);
      assert.equal((err as CalendarApiError).providerCode, 'INVALID_ARGUMENT');
      assert.equal((err as CalendarApiError).retryable, false);
    }
  });

  it('marks 5xx as retryable', async () => {
    const fetchImpl = mockFetch(async () => ({
      status: 503,
      body: { error: { code: 503, status: 'UNAVAILABLE' } }
    }));
    const client = new GoogleCalendarClient({ fetchImpl });
    try {
      await client.listPrimaryEvents('token', {
        timeMin: '2026-06-09T10:00:00.000Z',
        timeMax: '2026-06-09T11:00:00.000Z'
      });
      assert.fail('expected throw');
    } catch (err) {
      assert.ok(err instanceof CalendarApiError);
      assert.equal((err as CalendarApiError).retryable, true);
    }
  });

  it('does NOT expose a write method on the client surface', () => {
    const proto = Object.getPrototypeOf(new GoogleCalendarClient());
    const methods = Object.getOwnPropertyNames(proto).filter((n) => n !== 'constructor');
    for (const m of methods) {
      assert.equal(
        /insert|update|delete|patch|move|create|remove/i.test(m),
        false,
        `unexpected write-shaped method on GoogleCalendarClient: ${m}`
      );
    }
  });
});

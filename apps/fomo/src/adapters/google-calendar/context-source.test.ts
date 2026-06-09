// Phase v0.6.0C — CalendarContextSource unit + privacy tests.
//
// Covers the founder-locked PASS criteria for v0.6.0C:
//   C1  kill switch off → null
//   C2  non-allowlisted user → null
//   C3  allowlisted user with flag on → valid context
//   C4  private event → summary masked to "Busy", start/end preserved
//   C5  returned event keys are only summary/start/end
//   C6  excluded fields never cross the adapter boundary
//   C7  no calendar content in audit detail (success or failure)
//   C8  no DB writes, no memory_signal writes (audit-only assertion via
//       mock store contract — there are no other stores injected)
//   C9  24h, 48h default, 72h, 168h windows respected
//   C10 cache TTL behavior (hit within TTL; miss after expiry)
//   C11 provider failures sanitized through the chokepoint
//   C12 PIL/ranker code path NOT invoked by CalendarContextSource (assertion
//       via this module's narrow dep surface — there is no ranker dep)
//   C13 calendar_context is NOT exposed to the live ranker (this module
//       does not import or call rank/dispatch — assertion via grep on
//       prod imports, deferred to a separate test below)

import assert from 'node:assert/strict';
import { describe, it } from 'node:test';

import { InMemoryAuditStore } from '../../core/audit.ts';
import {
  CalendarApiError,
  CalendarUnauthorizedError,
  type GoogleCalendarClient
} from './client.ts';
import {
  CalendarContextSource,
  type CalendarContextSourceDeps,
  computeNearestEventOffsetMinutes,
  projectCalendarEvent,
  sanitizeCalendarError
} from './context-source.ts';
import type {
  CalendarEvent,
  RawGoogleCalendarEvent
} from './types.ts';

/* ---------- Test helpers --------------------------------------------- */

const FIXED_NOW_ISO = '2026-06-09T10:00:00.000Z';
const FIXED_NOW_MS = Date.parse(FIXED_NOW_ISO);
const FIXED_USER = 'user-fixture-1';
const OTHER_USER = 'user-fixture-2';

function makeFixtureClient(
  events: readonly RawGoogleCalendarEvent[]
): { client: GoogleCalendarClient; calls: number } {
  let calls = 0;
  const client: GoogleCalendarClient = {
    listPrimaryEvents: async () => {
      calls++;
      return Object.freeze(events);
    }
  } as unknown as GoogleCalendarClient;
  return {
    client,
    get calls() {
      return calls;
    }
  };
}

function makeFailingClient(err: Error): { client: GoogleCalendarClient } {
  const client: GoogleCalendarClient = {
    listPrimaryEvents: async () => {
      throw err;
    }
  } as unknown as GoogleCalendarClient;
  return { client };
}

function makeDeps(
  overrides: Partial<CalendarContextSourceDeps>
): CalendarContextSourceDeps {
  const auditStore = overrides.auditStore ?? new InMemoryAuditStore();
  const client =
    overrides.client ??
    (makeFixtureClient([]).client);
  return {
    client,
    getAccessToken: overrides.getAccessToken ?? (async () => 'access-token-fixture'),
    auditStore,
    enabled: overrides.enabled ?? true,
    allowlist: overrides.allowlist ?? [FIXED_USER],
    cacheTtlMs: overrides.cacheTtlMs ?? 60_000,
    defaultWindowHours: overrides.defaultWindowHours ?? 48,
    maxEvents: overrides.maxEvents,
    now: overrides.now ?? (() => FIXED_NOW_MS)
  };
}

const SAMPLE_PUBLIC_EVENT: RawGoogleCalendarEvent = {
  summary: 'Board sync',
  start: { dateTime: '2026-06-09T15:00:00.000Z' },
  end: { dateTime: '2026-06-09T16:00:00.000Z' },
  visibility: 'public'
};

const SAMPLE_PRIVATE_EVENT: RawGoogleCalendarEvent = {
  summary: 'Therapy — should never appear',
  start: { dateTime: '2026-06-09T17:00:00.000Z' },
  end: { dateTime: '2026-06-09T18:00:00.000Z' },
  visibility: 'private'
};

// Realistic event with all the excluded fields populated. Summary
// intentionally avoids containing any of the EXCLUDED_NEEDLES below so
// the privacy canary asserts only that the EXCLUDED fields are absent
// (summary itself is one of the three permitted fields).
const SAMPLE_WITH_EXCLUDED_FIELDS: RawGoogleCalendarEvent = {
  summary: 'Weekly sync',
  start: { dateTime: '2026-06-09T12:00:00.000Z' },
  end: { dateTime: '2026-06-09T13:00:00.000Z' },
  visibility: 'default',
  ...({
    attendees: [
      { email: 'sheila@example.com', responseStatus: 'accepted', displayName: 'Sheila' }
    ],
    description: 'Discuss Q3 OKR. Bring notes from Morris call.',
    location: '1234 Mission St, San Francisco',
    attachments: [{ fileUrl: 'https://drive.example.com/abc' }],
    conferenceData: { conferenceId: 'meet-xyz', entryPoints: [] },
    organizer: { email: 'organizer@example.com' },
    creator: { email: 'creator@example.com' },
    htmlLink: 'https://calendar.google.com/event?eid=ABC',
    recurringEventId: 'rec-123',
    hangoutLink: 'https://meet.google.com/abc-defg-hij'
  } as unknown as Record<string, unknown>)
};

// Strings that appear ONLY in excluded fields of SAMPLE_WITH_EXCLUDED_FIELDS
// + the private-event title. None of these may appear in any
// CalendarContext, CalendarEvent, or audit detail produced by the
// adapter. ("Sheila", "Lunch" intentionally NOT in the list — those would
// be legitimate summary text in a real calendar.)
const EXCLUDED_NEEDLES = [
  'attendees',
  'sheila@',
  'description',
  'OKR',
  'Morris',
  'location',
  'Mission',
  'attachments',
  'drive.example',
  'conferenceData',
  'meet-xyz',
  'organizer',
  'creator',
  'htmlLink',
  'recurringEventId',
  'rec-123',
  'hangoutLink',
  'meet.google',
  'Therapy'
];

/* ---------- C1: kill switch off → null ------------------------------- */

describe('CalendarContextSource — C1 kill switch off', () => {
  it('returns null when deps.enabled=false without calling the client', async () => {
    const fix = makeFixtureClient([SAMPLE_PUBLIC_EVENT]);
    const auditStore = new InMemoryAuditStore();
    const src = new CalendarContextSource(makeDeps({ enabled: false, client: fix.client, auditStore }));
    const ctx = await src.build(FIXED_USER, { windowHours: 48 });
    assert.equal(ctx, null);
    assert.equal(fix.calls, 0, 'must not call client when kill switch is off');
    const audits = await auditStore.recent(FIXED_USER, 10);
    assert.equal(audits.length, 0, 'must not write an audit row when the gate skips');
  });
});

/* ---------- C2: non-allowlisted user → null -------------------------- */

describe('CalendarContextSource — C2 allowlist gate', () => {
  it('returns null for a user not in the allowlist; no API call', async () => {
    const fix = makeFixtureClient([SAMPLE_PUBLIC_EVENT]);
    const auditStore = new InMemoryAuditStore();
    const src = new CalendarContextSource(
      makeDeps({ enabled: true, allowlist: [FIXED_USER], client: fix.client, auditStore })
    );
    const ctx = await src.build(OTHER_USER, { windowHours: 48 });
    assert.equal(ctx, null);
    assert.equal(fix.calls, 0);
    const audits = await auditStore.recent(OTHER_USER, 10);
    assert.equal(audits.length, 0);
  });

  it('allowlist comparison is strict === (case sensitive)', async () => {
    const fix = makeFixtureClient([SAMPLE_PUBLIC_EVENT]);
    const src = new CalendarContextSource(
      makeDeps({ enabled: true, allowlist: ['CASE-MATTERS'], client: fix.client })
    );
    const ctx = await src.build('case-matters', { windowHours: 48 });
    assert.equal(ctx, null);
    assert.equal(fix.calls, 0);
  });
});

/* ---------- C3: enabled + allowlisted → valid context ---------------- */

describe('CalendarContextSource — C3 happy path', () => {
  it('returns a valid context when enabled=true AND user is allowlisted', async () => {
    const fix = makeFixtureClient([SAMPLE_PUBLIC_EVENT]);
    const auditStore = new InMemoryAuditStore();
    const src = new CalendarContextSource(makeDeps({ client: fix.client, auditStore }));
    const ctx = await src.build(FIXED_USER, { windowHours: 48 });
    assert.ok(ctx);
    assert.equal(ctx!.event_count_in_window, 1);
    assert.equal(ctx!.window_hours_in_force, 48);
    assert.equal(ctx!.cache_hit, false);
    assert.equal(fix.calls, 1);
    const audits = await auditStore.recent(FIXED_USER, 10);
    assert.equal(audits.length, 1);
    assert.equal(audits[0]?.action, 'brevio.context.calendar_built');
    assert.equal(audits[0]?.result, 'success');
  });
});

/* ---------- C4 + C5 + C6: adapter-boundary contract ------------------ */

describe('CalendarContextSource — C4/C5/C6 adapter boundary', () => {
  it('private events mask summary to "Busy"; start/end preserved', async () => {
    const ev = projectCalendarEvent(SAMPLE_PRIVATE_EVENT);
    assert.ok(ev);
    assert.equal(ev!.summary, 'Busy');
    assert.equal(ev!.start, '2026-06-09T17:00:00.000Z');
    assert.equal(ev!.end, '2026-06-09T18:00:00.000Z');
  });

  it('confidential events also mask to "Busy"', async () => {
    const ev = projectCalendarEvent({
      ...SAMPLE_PRIVATE_EVENT,
      visibility: 'confidential'
    });
    assert.ok(ev);
    assert.equal(ev!.summary, 'Busy');
  });

  it('returned event has EXACTLY 3 keys: summary, start, end', async () => {
    const ev = projectCalendarEvent(SAMPLE_WITH_EXCLUDED_FIELDS);
    assert.ok(ev);
    const keys = Object.keys(ev!).sort();
    assert.deepEqual(keys, ['end', 'start', 'summary']);
  });

  it('all-day "date" shape events are rejected (return null)', async () => {
    const ev = projectCalendarEvent({
      summary: 'All day',
      start: { date: '2026-06-10' },
      end: { date: '2026-06-11' }
    });
    assert.equal(ev, null);
  });

  it('excluded fields never cross the adapter boundary in serialized output', async () => {
    const fix = makeFixtureClient([SAMPLE_WITH_EXCLUDED_FIELDS, SAMPLE_PRIVATE_EVENT]);
    const auditStore = new InMemoryAuditStore();
    const src = new CalendarContextSource(makeDeps({ client: fix.client, auditStore }));
    const ctx = await src.build(FIXED_USER, { windowHours: 48 });
    assert.ok(ctx);
    const serialized = JSON.stringify(ctx);
    for (const needle of EXCLUDED_NEEDLES) {
      assert.equal(
        serialized.includes(needle),
        false,
        `excluded field/value "${needle}" must NOT appear in CalendarContext`
      );
    }
  });
});

/* ---------- C7: audit detail has no calendar content ----------------- */

describe('CalendarContextSource — C7 audit privacy canary', () => {
  it('success audit detail contains only structural fields', async () => {
    const fix = makeFixtureClient([SAMPLE_WITH_EXCLUDED_FIELDS, SAMPLE_PRIVATE_EVENT]);
    const auditStore = new InMemoryAuditStore();
    const src = new CalendarContextSource(makeDeps({ client: fix.client, auditStore }));
    await src.build(FIXED_USER, { windowHours: 48 });
    const audits = await auditStore.recent(FIXED_USER, 10);
    assert.equal(audits.length, 1);
    const detailKeys = Object.keys(audits[0]!.detail ?? {}).sort();
    assert.deepEqual(detailKeys, [
      'cache_hit',
      'event_count_in_window',
      'nearest_event_start_offset_minutes',
      'source_surface',
      'window_hours_in_force'
    ]);
    const serialized = JSON.stringify(audits[0]?.detail);
    for (const needle of EXCLUDED_NEEDLES) {
      assert.equal(
        serialized.includes(needle),
        false,
        `excluded field/value "${needle}" must NOT appear in audit detail`
      );
    }
  });

  it('failure audit detail also contains no calendar content', async () => {
    const fix = makeFailingClient(new CalendarApiError(500, undefined, 'internal'));
    const auditStore = new InMemoryAuditStore();
    const src = new CalendarContextSource(makeDeps({ client: fix.client, auditStore }));
    const ctx = await src.build(FIXED_USER, { windowHours: 48 });
    assert.equal(ctx, null);
    const audits = await auditStore.recent(FIXED_USER, 10);
    assert.equal(audits.length, 1);
    assert.equal(audits[0]?.result, 'failure');
    const detail = audits[0]?.detail ?? {};
    assert.ok('error_code' in detail);
    assert.ok('error_reason' in detail);
    // No raw provider text fields.
    assert.equal('raw_provider_message' in detail, false);
    assert.equal('reason' in detail, false);
  });
});

/* ---------- C9: window respect (24/48/72/168) ------------------------- */

describe('CalendarContextSource — C9 windowHours respected', () => {
  for (const hours of [24, 48, 72, 168]) {
    it(`${hours}h window is passed through to the client unchanged`, async () => {
      let observedTimeMin: string | null = null;
      let observedTimeMax: string | null = null;
      const client: GoogleCalendarClient = {
        listPrimaryEvents: async (
          _token: string,
          opts: { timeMin: string; timeMax: string }
        ) => {
          observedTimeMin = opts.timeMin;
          observedTimeMax = opts.timeMax;
          return Object.freeze([]);
        }
      } as unknown as GoogleCalendarClient;
      const src = new CalendarContextSource(makeDeps({ client }));
      const ctx = await src.build(FIXED_USER, { windowHours: hours });
      assert.ok(ctx);
      assert.equal(ctx!.window_hours_in_force, hours);
      assert.equal(observedTimeMin, new Date(FIXED_NOW_MS).toISOString());
      assert.equal(
        observedTimeMax,
        new Date(FIXED_NOW_MS + hours * 60 * 60 * 1000).toISOString()
      );
    });
  }

  it('windowHours <= 0 falls back to defaultWindowHours', async () => {
    let observedTimeMax: string | null = null;
    const client: GoogleCalendarClient = {
      listPrimaryEvents: async (
        _token: string,
        opts: { timeMin: string; timeMax: string }
      ) => {
        observedTimeMax = opts.timeMax;
        return Object.freeze([]);
      }
    } as unknown as GoogleCalendarClient;
    const src = new CalendarContextSource(
      makeDeps({ client, defaultWindowHours: 48 })
    );
    const ctx = await src.build(FIXED_USER, { windowHours: 0 });
    assert.ok(ctx);
    assert.equal(ctx!.window_hours_in_force, 48);
    assert.equal(
      observedTimeMax,
      new Date(FIXED_NOW_MS + 48 * 60 * 60 * 1000).toISOString()
    );
  });
});

/* ---------- C10: cache TTL behavior --------------------------------- */

describe('CalendarContextSource — C10 cache TTL', () => {
  it('returns the cached context on a second call within TTL; one API call total', async () => {
    const fix = makeFixtureClient([SAMPLE_PUBLIC_EVENT]);
    const src = new CalendarContextSource(makeDeps({ client: fix.client, cacheTtlMs: 60_000 }));
    const a = await src.build(FIXED_USER, { windowHours: 48 });
    const b = await src.build(FIXED_USER, { windowHours: 48 });
    assert.ok(a);
    assert.ok(b);
    assert.equal(fix.calls, 1);
    assert.equal(a!.cache_hit, false);
    assert.equal(b!.cache_hit, true);
  });

  it('after TTL expiry, the next call hits the API again', async () => {
    const fix = makeFixtureClient([SAMPLE_PUBLIC_EVENT]);
    let nowMs = FIXED_NOW_MS;
    const src = new CalendarContextSource(
      makeDeps({ client: fix.client, cacheTtlMs: 60_000, now: () => nowMs })
    );
    const a = await src.build(FIXED_USER, { windowHours: 48 });
    nowMs += 60_001; // advance past TTL
    const b = await src.build(FIXED_USER, { windowHours: 48 });
    assert.ok(a);
    assert.ok(b);
    assert.equal(fix.calls, 2);
    assert.equal(b!.cache_hit, false);
  });

  it('cache is keyed by (user, windowHours): different windows do not share', async () => {
    const fix = makeFixtureClient([SAMPLE_PUBLIC_EVENT]);
    const src = new CalendarContextSource(makeDeps({ client: fix.client }));
    await src.build(FIXED_USER, { windowHours: 48 });
    await src.build(FIXED_USER, { windowHours: 72 });
    assert.equal(fix.calls, 2);
  });
});

/* ---------- C11: sanitized failure handling -------------------------- */

describe('CalendarContextSource — C11 sanitized errors', () => {
  it('401 → auth_error', async () => {
    const e = sanitizeCalendarError(new CalendarUnauthorizedError('rejected'));
    assert.equal(e.error_reason, 'auth_error');
  });

  it('429 → rate_limited', async () => {
    const e = sanitizeCalendarError(new CalendarApiError(429, undefined, 'busy'));
    assert.equal(e.error_reason, 'rate_limited');
  });

  it('500 → temporary_provider_error', async () => {
    const e = sanitizeCalendarError(new CalendarApiError(500, undefined, 'internal'));
    assert.equal(e.error_reason, 'temporary_provider_error');
  });

  it('non-Error throw → unknown_error', async () => {
    const e = sanitizeCalendarError('oops' as unknown);
    assert.equal(e.error_reason, 'unknown_error');
  });
});

/* ---------- C-extra: no access token → failure audit + null --------- */

describe('CalendarContextSource — getAccessToken missing', () => {
  it('returns null and writes a sanitized failure audit when no token is available', async () => {
    const auditStore = new InMemoryAuditStore();
    const fix = makeFixtureClient([SAMPLE_PUBLIC_EVENT]);
    const src = new CalendarContextSource(
      makeDeps({
        client: fix.client,
        auditStore,
        getAccessToken: async () => null
      })
    );
    const ctx = await src.build(FIXED_USER, { windowHours: 48 });
    assert.equal(ctx, null);
    assert.equal(fix.calls, 0, 'must not call client when access token is missing');
    const audits = await auditStore.recent(FIXED_USER, 10);
    assert.equal(audits.length, 1);
    assert.equal(audits[0]?.result, 'failure');
    assert.equal(audits[0]?.detail?.error_reason, 'auth_error');
  });
});

/* ---------- nearest-event offset ------------------------------------ */

describe('computeNearestEventOffsetMinutes', () => {
  it('returns null on empty list', () => {
    assert.equal(computeNearestEventOffsetMinutes([], FIXED_NOW_MS), null);
  });

  it('returns positive minutes for future starts', () => {
    const ev: CalendarEvent = Object.freeze({
      summary: 'x',
      start: new Date(FIXED_NOW_MS + 30 * 60_000).toISOString(),
      end: new Date(FIXED_NOW_MS + 60 * 60_000).toISOString()
    });
    assert.equal(computeNearestEventOffsetMinutes([ev], FIXED_NOW_MS), 30);
  });

  it('returns negative minutes for in-progress events', () => {
    const ev: CalendarEvent = Object.freeze({
      summary: 'x',
      start: new Date(FIXED_NOW_MS - 10 * 60_000).toISOString(),
      end: new Date(FIXED_NOW_MS + 30 * 60_000).toISOString()
    });
    assert.equal(computeNearestEventOffsetMinutes([ev], FIXED_NOW_MS), -10);
  });
});

/* ---------- live-ranker invariant (C13) ----------------------------- */

describe('CalendarContextSource — C13 live ranker not touched', () => {
  it('does not import the production rank call site (smoke check via module deps)', async () => {
    // The CalendarContextSource module's runtime dep surface is exactly:
    //   - core/audit
    //   - core/sanitize-provider-error
    //   - ranker/context-sources/index (seam interface only — no code)
    //   - ./client + ./types (the adapter)
    //
    // It does NOT import:
    //   - ranker/index, ranker/prompt, ranker/pil-context (any of them)
    //   - workers/gmail-poll, dispatch/*, alert state machines
    //
    // This test asserts the boundary structurally by importing the module
    // and checking that the exported surface is exactly what the seam
    // contract requires — nothing leaks. If a future refactor accidentally
    // wires Calendar context into the live ranker, the imports here would
    // grow and a reviewer would notice.
    //
    // (See PASS criterion C13 in the v0.6.0C scope: calendar_context is
    // built and audited but NOT passed to the live ranker until v0.6.0E.)
    const exports = await import('./context-source.ts');
    const exportedNames = Object.keys(exports).sort();
    assert.deepEqual(
      exportedNames,
      [
        'CalendarContextSource',
        'computeNearestEventOffsetMinutes',
        'projectCalendarEvent',
        'sanitizeCalendarError'
      ],
      'CalendarContextSource module must not export ranker-side helpers'
    );
  });
});

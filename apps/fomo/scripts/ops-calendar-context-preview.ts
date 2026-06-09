// Phase v0.6.0C — Calendar context preview script.
//
// DETERMINISTIC, PURE FUNCTION. NO LIVE CALENDAR API CALL. NO PIL OR
// RANKER CHANGES. NO LIVE OAUTH TOKEN ACCESS. Satisfies the §6 product-
// visibility requirement of [docs/v0.6.0B-oauth-scope-readiness.md].
//
// Takes a fixture set of raw Google Calendar events (representative
// shape — see CALENDAR_FIXTURE below) and prints:
//
//   1. The projection at the adapter boundary (summary/start/end only;
//      private-event "Busy" mask applied)
//   2. The CalendarContext structural fields the audit would carry
//   3. The compact prompt block a future ranker prompt would receive
//
// Founder reads this output to confirm "what Calendar adds" makes sense
// before v0.6.0E flips Calendar into the live ranker.
//
// Run:
//   pnpm --filter @brevio/fomo ops:calendar-context-preview

import {
  computeNearestEventOffsetMinutes,
  projectCalendarEvent
} from '../src/adapters/google-calendar/context-source.js';
import type {
  CalendarEvent,
  RawGoogleCalendarEvent
} from '../src/adapters/google-calendar/types.js';

// FIXED clock so the preview output is reproducible across runs.
const FIXED_NOW_ISO = '2026-06-09T10:00:00.000Z';
const FIXED_NOW_MS = Date.parse(FIXED_NOW_ISO);
const WINDOW_HOURS = 48;

// Fixture: 4 events covering the boundary cases v0.6.0C handles. Each
// event includes fields beyond summary/start/end (visibility, attendees,
// description, location, conferenceData) so the projection step can
// demonstrate field exclusion.
const CALENDAR_FIXTURE: readonly RawGoogleCalendarEvent[] = [
  {
    summary: '1:1 with Galiette',
    start: { dateTime: '2026-06-09T14:00:00.000Z' },
    end: { dateTime: '2026-06-09T14:30:00.000Z' },
    visibility: 'default',
    // Fields below are intentionally present so the projection step
    // demonstrably discards them. They never reach the CalendarEvent.
    ...({
      attendees: [{ email: 'galiette@example.com', responseStatus: 'accepted' }],
      description: 'Weekly sync. Bring product notes.',
      location: 'Brevio HQ, Floor 4',
      conferenceData: { conferenceId: 'abc-123', entryPoints: [] }
    } as unknown as Record<string, unknown>)
  },
  {
    summary: 'Therapy appointment',
    start: { dateTime: '2026-06-09T19:00:00.000Z' },
    end: { dateTime: '2026-06-09T20:00:00.000Z' },
    visibility: 'private',
    ...({
      attendees: [{ email: 'doctor@example.com' }],
      description: 'Private notes — should never appear in output'
    } as unknown as Record<string, unknown>)
  },
  {
    summary: 'Board meeting',
    start: { dateTime: '2026-06-10T15:00:00.000Z' },
    end: { dateTime: '2026-06-10T16:30:00.000Z' },
    visibility: 'public'
  },
  {
    // All-day event with `date` instead of `dateTime` — the adapter
    // boundary intentionally rejects this shape in v0.6.0C scope.
    summary: 'Conference travel day',
    start: { date: '2026-06-11' },
    end: { date: '2026-06-12' },
    visibility: 'default'
  }
];

function formatEvent(ev: CalendarEvent): string {
  return `  • ${ev.summary}\n      ${ev.start} → ${ev.end}`;
}

function main(): void {
  const projected: CalendarEvent[] = [];
  for (const raw of CALENDAR_FIXTURE) {
    const ev = projectCalendarEvent(raw);
    if (ev !== null) projected.push(ev);
  }
  const nearestMinutes = computeNearestEventOffsetMinutes(projected, FIXED_NOW_MS);

  process.stdout.write('=== v0.6.0C — Calendar context preview ===\n\n');
  process.stdout.write(`Fixed clock:   ${FIXED_NOW_ISO}\n`);
  process.stdout.write(`Window:        ${WINDOW_HOURS} hours\n`);
  process.stdout.write(`Fixture size:  ${CALENDAR_FIXTURE.length} raw events\n`);
  process.stdout.write(`After project: ${projected.length} events\n`);
  process.stdout.write(
    `(Rejected: events with missing dateTime — all-day "date" shapes are out of scope in v0.6.0C.)\n\n`
  );

  process.stdout.write('--- Adapter boundary projection (summary/start/end only) ---\n');
  for (const ev of projected) {
    process.stdout.write(formatEvent(ev) + '\n');
  }
  process.stdout.write('\n');

  process.stdout.write('--- brevio.context.calendar_built audit detail (structural only) ---\n');
  process.stdout.write(
    JSON.stringify(
      {
        event_count_in_window: projected.length,
        nearest_event_start_offset_minutes: nearestMinutes,
        window_hours_in_force: WINDOW_HOURS,
        source_surface: 'email_alert',
        cache_hit: false
      },
      null,
      2
    ) + '\n\n'
  );

  process.stdout.write(
    '--- Prompt block a future ranker MIGHT receive (NOT wired in v0.6.0C) ---\n'
  );
  if (projected.length === 0) {
    process.stdout.write('(no events in window)\n');
  } else {
    process.stdout.write(`Calendar (next ${WINDOW_HOURS}h):\n`);
    for (const ev of projected) {
      const startMinutes = Math.round((Date.parse(ev.start) - FIXED_NOW_MS) / 60_000);
      const offsetLabel =
        startMinutes < 0
          ? `${Math.abs(startMinutes)}m in progress`
          : startMinutes < 60
            ? `in ${startMinutes}m`
            : `in ${Math.floor(startMinutes / 60)}h${startMinutes % 60 ? ` ${startMinutes % 60}m` : ''}`;
      process.stdout.write(`  ${offsetLabel}: ${ev.summary}\n`);
    }
  }
  process.stdout.write('\n');

  process.stdout.write(
    '--- Privacy canary (must NEVER appear in any output above) ---\n'
  );
  const banned = ['attendees', 'description', 'location', 'conferenceData', 'doctor@', 'Floor 4'];
  for (const needle of banned) {
    const hit = JSON.stringify(projected).includes(needle);
    process.stdout.write(`  "${needle}": ${hit ? 'PRESENT — FAIL' : 'absent — OK'}\n`);
  }
  process.stdout.write('\n');

  process.stdout.write('NOTE: Calendar context is NOT wired into the live ranker in v0.6.0C.\n');
  process.stdout.write('      v0.6.0E is the phase that runs its own gate before that step.\n');
}

main();

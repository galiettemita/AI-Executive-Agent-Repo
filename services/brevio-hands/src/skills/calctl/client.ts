import type { CalctlEvent, CalctlInput, CalctlOutput } from './types.js';

const BASE_EVENTS: CalctlEvent[] = [
  {
    event_id: 'evt-leadership-sync',
    title: 'Leadership Sync',
    start_at: '2026-03-05T16:00:00.000Z',
    end_at: '2026-03-05T16:30:00.000Z',
    calendar: 'Work',
    status: 'confirmed'
  },
  {
    event_id: 'evt-dinner-family',
    title: 'Family Dinner',
    start_at: '2026-03-05T23:00:00.000Z',
    end_at: '2026-03-06T00:00:00.000Z',
    calendar: 'Personal',
    status: 'confirmed'
  }
];

function eventFromInput(input: CalctlInput): CalctlEvent {
  return {
    event_id: input.event_id ?? `evt-${(input.title ?? 'new-event').toLowerCase().replace(/[^a-z0-9]+/g, '-').slice(0, 20)}`,
    title: input.title ?? 'Untitled event',
    start_at: input.start_at ?? '2026-03-06T15:00:00.000Z',
    end_at: input.end_at ?? '2026-03-06T15:30:00.000Z',
    calendar: input.calendar ?? 'Work',
    status: input.action === 'cancel_event' ? 'cancelled' : 'confirmed'
  };
}

export async function runClient(input: CalctlInput): Promise<CalctlOutput> {
  if (input.action === 'list_events') {
    return {
      provider: 'apple-calendar',
      action: input.action,
      events: BASE_EVENTS,
      summary: `Found ${BASE_EVENTS.length} events in active calendars.`
    };
  }

  const event = eventFromInput(input);
  const verb =
    input.action === 'create_event'
      ? 'Created'
      : input.action === 'update_event'
        ? 'Updated'
        : 'Cancelled';

  return {
    provider: 'apple-calendar',
    action: input.action,
    events: [event],
    summary: `${verb} event "${event.title}" on ${event.calendar}.`
  };
}

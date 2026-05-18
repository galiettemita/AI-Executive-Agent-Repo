import type {
  CalendarEventOutput,
  GoogleCalendarInput,
  GoogleCalendarOutput
} from './types.js';

function deterministicEventId(seed: string): string {
  let hash = 2166136261;
  for (let i = 0; i < seed.length; i += 1) {
    hash ^= seed.charCodeAt(i);
    hash +=
      (hash << 1) +
      (hash << 4) +
      (hash << 7) +
      (hash << 8) +
      (hash << 24);
  }
  const hex = (hash >>> 0).toString(16).padStart(8, '0');
  return `evt_${hex}`;
}

function defaultEvents(calendarId: string): CalendarEventOutput[] {
  const now = new Date();
  const start = new Date(now.getTime() + 60 * 60 * 1000).toISOString();
  const end = new Date(now.getTime() + 90 * 60 * 1000).toISOString();

  return [
    {
      event_id: deterministicEventId(`${calendarId}:1`),
      title: 'Executive sync',
      start_time: start,
      end_time: end,
      status: 'scheduled'
    },
    {
      event_id: deterministicEventId(`${calendarId}:2`),
      title: 'Planning block',
      start_time: new Date(now.getTime() + 3 * 60 * 60 * 1000).toISOString(),
      end_time: new Date(now.getTime() + 4 * 60 * 60 * 1000).toISOString(),
      status: 'scheduled'
    }
  ];
}

export async function runClient(input: GoogleCalendarInput): Promise<GoogleCalendarOutput> {
  const calendarId = input.calendar_id ?? 'primary';

  if (input.action === 'list') {
    return {
      action: 'list',
      calendar_id: calendarId,
      events: defaultEvents(calendarId),
      confirmation_required: false
    };
  }

  if ((input.action === 'create' || input.action === 'delete') && !input.confirmed) {
    return {
      action: input.action,
      calendar_id: calendarId,
      confirmation_required: true
    };
  }

  if (input.action === 'create') {
    const eventId = deterministicEventId(`${calendarId}:${input.event?.title ?? 'event'}`);
    return {
      action: 'create',
      calendar_id: calendarId,
      event_id: eventId,
      confirmation_required: false
    };
  }

  if (input.action === 'update') {
    return {
      action: 'update',
      calendar_id: calendarId,
      event_id: input.event?.event_id ?? deterministicEventId(`${calendarId}:update`),
      confirmation_required: false
    };
  }

  return {
    action: 'delete',
    calendar_id: calendarId,
    event_id: input.event?.event_id ?? deterministicEventId(`${calendarId}:delete`),
    confirmation_required: false
  };
}

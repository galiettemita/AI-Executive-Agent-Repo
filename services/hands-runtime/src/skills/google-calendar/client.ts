// Plan §6 steps 20–21 — Real Google Calendar /calendars/primary/events
// OAuth: ctx.token||GOOGLE_CALENDAR_TOKEN per plan §6 step 20 and §4 architecture

import type { GoogleCalendarInput, GoogleCalendarOutput } from './types.js';

const GCAL_BASE = 'https://www.googleapis.com/calendar/v3';

interface GCalEventItem {
  id: string;
  summary?: string;
  start?: { dateTime?: string; date?: string };
  end?: { dateTime?: string; date?: string };
}

interface GCalListResponse {
  items?: GCalEventItem[];
}

interface GCalCreateResponse {
  id: string;
}

// ctx carries the per-user OAuth token injected by Go credential_resolver (plan §4)
export async function runClient(
  input: GoogleCalendarInput,
  ctx?: { token?: string }
): Promise<GoogleCalendarOutput> {
  const token = ctx?.token || process.env.GOOGLE_CALENDAR_TOKEN;
  if (!token) throw new Error('google-calendar: GOOGLE_CALENDAR_TOKEN not set');

  const calendarId = input.calendar_id ?? 'primary';

  const headers = {
    'Authorization': `Bearer ${token}`,
    'Content-Type': 'application/json',
  };

  if (input.action === 'list') {
    // Plan §6 step 20: exact query params
    const url = new URL(`${GCAL_BASE}/calendars/${calendarId}/events`);
    url.searchParams.set('maxResults', '20');
    url.searchParams.set('singleEvents', 'true');
    url.searchParams.set('orderBy', 'startTime');
    url.searchParams.set('timeMin', new Date().toISOString());

    const response = await fetch(url.toString(), {
      headers,
      signal: AbortSignal.timeout(10000),
    });

    if (!response.ok) {
      const text = await response.text();
      throw new Error(`google-calendar: HTTP ${response.status} – ${text.slice(0, 300)}`);
    }

    const data = (await response.json()) as GCalListResponse;

    return {
      action: 'list',
      calendar_id: calendarId,
      confirmation_required: false,
      events: (data.items ?? []).map((e) => ({
        event_id: e.id,
        title: e.summary ?? '',
        start_time: e.start?.dateTime ?? e.start?.date ?? '',
        end_time: e.end?.dateTime ?? e.end?.date ?? '',
        status: 'scheduled' as const,
      })),
    };
  }

  if (input.action === 'create') {
    // Plan §6 step 21: confirmation gate
    if (!input.confirmed) {
      return {
        action: 'create',
        calendar_id: calendarId,
        confirmation_required: true,
      };
    }

    const response = await fetch(`${GCAL_BASE}/calendars/${calendarId}/events`, {
      method: 'POST',
      headers,
      body: JSON.stringify({
        summary: input.event?.title ?? '',
        start: { dateTime: input.event?.start_time },
        end: { dateTime: input.event?.end_time },
        description: input.event?.description ?? '',
        location: input.event?.location ?? '',
      }),
      signal: AbortSignal.timeout(10000),
    });

    if (!response.ok) {
      const text = await response.text();
      throw new Error(`google-calendar: HTTP ${response.status} – ${text.slice(0, 300)}`);
    }

    const data = (await response.json()) as GCalCreateResponse;
    return {
      action: 'create',
      calendar_id: calendarId,
      event_id: data.id,
      confirmation_required: false,
    };
  }

  if (input.action === 'update') {
    if (!input.event?.event_id) {
      throw new Error('google-calendar: event_id is required for update');
    }

    const response = await fetch(
      `${GCAL_BASE}/calendars/${calendarId}/events/${input.event.event_id}`,
      {
        method: 'PATCH',
        headers,
        body: JSON.stringify({
          summary: input.event?.title,
          start: input.event?.start_time ? { dateTime: input.event.start_time } : undefined,
          end: input.event?.end_time ? { dateTime: input.event.end_time } : undefined,
          description: input.event?.description,
          location: input.event?.location,
        }),
        signal: AbortSignal.timeout(10000),
      }
    );

    if (!response.ok) {
      const text = await response.text();
      throw new Error(`google-calendar: HTTP ${response.status} – ${text.slice(0, 300)}`);
    }

    return {
      action: 'update',
      calendar_id: calendarId,
      event_id: input.event.event_id,
      confirmation_required: false,
    };
  }

  if (input.action === 'delete') {
    if (!input.confirmed) {
      return {
        action: 'delete',
        calendar_id: calendarId,
        confirmation_required: true,
      };
    }

    if (!input.event?.event_id) {
      throw new Error('google-calendar: event_id is required for delete');
    }

    const response = await fetch(
      `${GCAL_BASE}/calendars/${calendarId}/events/${input.event.event_id}`,
      {
        method: 'DELETE',
        headers,
        signal: AbortSignal.timeout(10000),
      }
    );

    if (response.status !== 204 && !response.ok) {
      const text = await response.text().catch(() => '');
      throw new Error(`google-calendar: HTTP ${response.status} – ${text.slice(0, 300)}`);
    }

    return {
      action: 'delete',
      calendar_id: calendarId,
      event_id: input.event.event_id,
      confirmation_required: false,
    };
  }

  throw new Error(`google-calendar: unknown action ${input.action}`);
}

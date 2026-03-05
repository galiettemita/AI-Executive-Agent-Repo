export type CalendarAction = 'list' | 'create' | 'update' | 'delete';

export interface CalendarEventInput {
  event_id?: string;
  title?: string;
  start_time?: string;
  end_time?: string;
  description?: string;
  location?: string;
}

export interface GoogleCalendarInput {
  action: CalendarAction;
  calendar_id?: string;
  confirmed?: boolean;
  event?: CalendarEventInput;
}

export interface CalendarEventOutput {
  event_id: string;
  title: string;
  start_time: string;
  end_time: string;
  status: 'scheduled' | 'updated' | 'deleted';
}

export interface GoogleCalendarOutput {
  action: CalendarAction;
  calendar_id: string;
  events?: CalendarEventOutput[];
  event_id?: string;
  confirmation_required: boolean;
}

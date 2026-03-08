export type CalctlAction = 'create_event' | 'list_events' | 'update_event' | 'cancel_event';

export interface CalctlInput {
  action: CalctlAction;
  event_id?: string;
  title?: string;
  start_at?: string;
  end_at?: string;
  calendar?: string;
}

export interface CalctlEvent {
  event_id: string;
  title: string;
  start_at: string;
  end_at: string;
  calendar: string;
  status: 'confirmed' | 'cancelled';
}

export interface CalctlOutput {
  provider: 'apple-calendar';
  action: CalctlAction;
  events: CalctlEvent[];
  summary: string;
}

export type OutlookAction = 'inbox_list' | 'send' | 'calendar_list';

export interface OutlookInput {
  action: OutlookAction;
  to?: string[];
  subject?: string;
  body?: string;
  confirmed?: boolean;
}

export interface OutlookMail {
  message_id: string;
  from: string;
  subject: string;
}

export interface OutlookEvent {
  event_id: string;
  subject: string;
  start_time: string;
}

export interface OutlookOutput {
  provider: 'outlook';
  action: OutlookAction;
  confirmation_required?: boolean;
  message_id?: string;
  mails?: OutlookMail[];
  events?: OutlookEvent[];
}

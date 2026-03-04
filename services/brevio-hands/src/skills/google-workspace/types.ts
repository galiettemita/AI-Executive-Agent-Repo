export type GoogleWorkspaceAction =
  | 'gmail_list'
  | 'gmail_send'
  | 'calendar_list'
  | 'drive_search';

export interface GoogleWorkspaceInput {
  action: GoogleWorkspaceAction;
  query?: string;
  to?: string[];
  subject?: string;
  body?: string;
  confirmed?: boolean;
}

export interface GoogleWorkspaceMail {
  message_id: string;
  from: string;
  subject: string;
}

export interface GoogleWorkspaceCalendarEvent {
  event_id: string;
  title: string;
  start_time: string;
}

export interface GoogleWorkspaceDriveFile {
  file_id: string;
  name: string;
  mime_type: string;
}

export interface GoogleWorkspaceOutput {
  provider: 'google-workspace';
  action: GoogleWorkspaceAction;
  confirmation_required?: boolean;
  message_id?: string;
  mails?: GoogleWorkspaceMail[];
  events?: GoogleWorkspaceCalendarEvent[];
  files?: GoogleWorkspaceDriveFile[];
}

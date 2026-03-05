export interface ImapEmailInput {
  action: 'list' | 'search' | 'send';
  mailbox?: string;
  query?: string;
  to?: string[];
  subject?: string;
  body?: string;
  confirmed?: boolean;
}

export interface ImapEmailMessage {
  id: string;
  from: string;
  subject: string;
  snippet: string;
  received_at: string;
}

export interface ImapEmailOutput {
  provider: 'imap-email';
  action: ImapEmailInput['action'];
  mailbox: string;
  messages?: ImapEmailMessage[];
  sent?: boolean;
}

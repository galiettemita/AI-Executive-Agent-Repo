export interface AppleMailInput {
  action: 'list_inbox' | 'search' | 'send' | 'reply';
  query?: string;
  to?: string[];
  subject?: string;
  body?: string;
  reply_to_id?: string;
  confirmed?: boolean;
}

export interface AppleMailMessage {
  id: string;
  from: string;
  to: string[];
  subject: string;
  snippet: string;
  received_at: string;
}

export interface AppleMailOutput {
  provider: 'apple-mail-local';
  action: AppleMailInput['action'];
  emails?: AppleMailMessage[];
  sent?: boolean;
  message_id?: string;
}

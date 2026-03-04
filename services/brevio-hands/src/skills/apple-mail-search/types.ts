export type AppleMailSearchAction = 'search_all' | 'search_sender' | 'search_subject';

export interface AppleMailSearchInput {
  action: AppleMailSearchAction;
  query?: string;
  mailbox?: string;
  limit?: number;
}

export interface AppleMailSearchResult {
  message_id: string;
  mailbox: string;
  sender: string;
  subject: string;
  snippet: string;
  received_at: string;
}

export interface AppleMailSearchOutput {
  provider: 'apple-mail-search';
  action: AppleMailSearchAction;
  query: string;
  results: AppleMailSearchResult[];
  latency_profile_ms: 50;
  summary: string;
}

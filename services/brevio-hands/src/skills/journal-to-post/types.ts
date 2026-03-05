export type JournalToPostAction = 'draft_post' | 'generate_thread';

export interface JournalToPostInput {
  action: JournalToPostAction;
  journal_entry?: string;
  platform?: 'x' | 'linkedin' | 'bluesky';
}

export interface JournalToPostOutput {
  provider: 'journal-to-post';
  action: JournalToPostAction;
  platform: 'x' | 'linkedin' | 'bluesky';
  post_text: string;
  thread_parts: string[];
  summary: string;
}

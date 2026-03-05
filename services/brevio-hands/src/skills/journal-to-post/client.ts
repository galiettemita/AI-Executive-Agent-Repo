import type { JournalToPostInput, JournalToPostOutput } from './types.js';

export async function runClient(input: JournalToPostInput): Promise<JournalToPostOutput> {
  const platform = input.platform ?? 'x';
  const entry = input.journal_entry ?? '';
  const post = entry.slice(0, 240);
  const thread = [post, 'Key takeaway: keep the main lesson actionable.', 'What would you add?'];

  return {
    provider: 'journal-to-post',
    action: input.action,
    platform,
    post_text: post,
    thread_parts: input.action === 'generate_thread' ? thread : [post],
    summary: `Prepared ${input.action === 'generate_thread' ? 'thread' : 'single post'} for ${platform}.`
  };
}

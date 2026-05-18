import type { AppleMailSearchInput, AppleMailSearchOutput } from './types.js';

const SAMPLE_RESULTS: AppleMailSearchOutput['results'] = [
  {
    message_id: 'mail_001',
    mailbox: 'Inbox',
    sender: 'ceo@partner.com',
    subject: 'Q2 roadmap sync',
    snippet: 'Can we lock 11:00am tomorrow for the roadmap review?',
    received_at: '2026-03-04T12:10:00.000Z'
  },
  {
    message_id: 'mail_002',
    mailbox: 'Inbox',
    sender: 'finance@company.com',
    subject: 'Board packet updates',
    snippet: 'Attached are the revised financial schedules for the board packet.',
    received_at: '2026-03-04T10:35:00.000Z'
  }
];

export async function runClient(input: AppleMailSearchInput): Promise<AppleMailSearchOutput> {
  const query = input.query ?? '';
  const results = SAMPLE_RESULTS.filter((result) => {
    if (input.action === 'search_sender') {
      return result.sender.toLowerCase().includes(query.toLowerCase());
    }

    if (input.action === 'search_subject') {
      return result.subject.toLowerCase().includes(query.toLowerCase());
    }

    return (
      result.subject.toLowerCase().includes(query.toLowerCase()) ||
      result.snippet.toLowerCase().includes(query.toLowerCase()) ||
      result.sender.toLowerCase().includes(query.toLowerCase())
    );
  }).slice(0, input.limit ?? 10);

  return {
    provider: 'apple-mail-search',
    action: input.action,
    query,
    results,
    latency_profile_ms: 50,
    summary: `Found ${results.length} Apple Mail result(s) for query "${query}".`
  };
}

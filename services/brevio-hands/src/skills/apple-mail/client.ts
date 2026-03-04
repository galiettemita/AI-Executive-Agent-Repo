import type { AppleMailInput, AppleMailMessage, AppleMailOutput } from './types.js';

const INBOX: AppleMailMessage[] = [
  {
    id: 'mail_001',
    from: 'sarah@example.com',
    to: ['exec@example.com'],
    subject: 'Board prep notes',
    snippet: 'Attaching the latest board deck edits for review.',
    received_at: '2026-03-04T14:11:00.000Z'
  },
  {
    id: 'mail_002',
    from: 'ops@example.com',
    to: ['exec@example.com'],
    subject: 'Travel updates',
    snippet: 'Flight AA100 is delayed by 35 minutes.',
    received_at: '2026-03-04T12:40:00.000Z'
  }
];

export async function runClient(input: AppleMailInput): Promise<AppleMailOutput> {
  if (input.action === 'list_inbox') {
    return {
      provider: 'apple-mail-local',
      action: 'list_inbox',
      emails: INBOX
    };
  }

  if (input.action === 'search') {
    const terms = (input.query ?? '').toLowerCase().split(/\s+/u).filter((term) => term.length > 1);
    const emails = INBOX.filter((email) => {
      const haystack = `${email.subject} ${email.snippet} ${email.from}`.toLowerCase();
      return terms.some((term) => haystack.includes(term));
    });
    return {
      provider: 'apple-mail-local',
      action: 'search',
      emails
    };
  }

  if (input.action === 'send') {
    return {
      provider: 'apple-mail-local',
      action: 'send',
      sent: true,
      message_id: 'mail_sent_apple_001'
    };
  }

  return {
    provider: 'apple-mail-local',
    action: 'reply',
    sent: true,
    message_id: 'mail_reply_apple_001'
  };
}

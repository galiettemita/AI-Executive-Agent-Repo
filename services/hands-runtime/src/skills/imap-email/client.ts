import type { ImapEmailInput, ImapEmailMessage, ImapEmailOutput } from './types.js';

const MAILBOX: ImapEmailMessage[] = [
  {
    id: 'imap_001',
    from: 'finance@example.com',
    subject: 'Invoice reminder',
    snippet: 'Reminder that invoice 3821 is due this Friday.',
    received_at: '2026-03-04T13:05:00.000Z'
  },
  {
    id: 'imap_002',
    from: 'assistant@example.com',
    subject: 'Meeting recap',
    snippet: 'Here are your action items from today\'s call.',
    received_at: '2026-03-04T11:20:00.000Z'
  }
];

export async function runClient(input: ImapEmailInput): Promise<ImapEmailOutput> {
  const mailbox = input.mailbox ?? 'INBOX';

  if (input.action === 'list') {
    return {
      provider: 'imap-email',
      action: 'list',
      mailbox,
      messages: MAILBOX
    };
  }

  if (input.action === 'search') {
    const terms = (input.query ?? '').toLowerCase().split(/\s+/u).filter((term) => term.length > 1);
    const messages = MAILBOX.filter((message) => {
      const haystack = `${message.subject} ${message.snippet} ${message.from}`.toLowerCase();
      return terms.some((term) => haystack.includes(term));
    });

    return {
      provider: 'imap-email',
      action: 'search',
      mailbox,
      messages
    };
  }

  return {
    provider: 'imap-email',
    action: 'send',
    mailbox,
    sent: true
  };
}

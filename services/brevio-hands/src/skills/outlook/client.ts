import { createHash } from 'node:crypto';

import type { OutlookEvent, OutlookInput, OutlookMail, OutlookOutput } from './types.js';

const MAILS: OutlookMail[] = [
  {
    message_id: 'out_msg_001',
    from: 'finance@example.com',
    subject: 'Invoice approvals'
  },
  {
    message_id: 'out_msg_002',
    from: 'partner@example.com',
    subject: 'Contract revision notes'
  }
];

const EVENTS: OutlookEvent[] = [
  {
    event_id: 'out_evt_001',
    subject: 'Vendor sync',
    start_time: '2026-03-06T15:00:00.000Z'
  },
  {
    event_id: 'out_evt_002',
    subject: 'Board prep call',
    start_time: '2026-03-06T18:30:00.000Z'
  }
];

function buildMessageID(input: OutlookInput): string {
  const digest = createHash('sha256')
    .update((input.to ?? []).join(','))
    .update('|')
    .update(input.subject ?? '')
    .update('|')
    .update((input.body ?? '').slice(0, 120))
    .digest('hex')
    .slice(0, 16);
  return `out_${digest}`;
}

export async function runClient(input: OutlookInput): Promise<OutlookOutput> {
  if (input.action === 'inbox_list') {
    return {
      provider: 'outlook',
      action: 'inbox_list',
      mails: MAILS
    };
  }

  if (input.action === 'calendar_list') {
    return {
      provider: 'outlook',
      action: 'calendar_list',
      events: EVENTS
    };
  }

  if (!input.to || !input.subject || !input.body) {
    throw new Error('OUTLOOK_SEND_FIELDS_REQUIRED');
  }

  if (!input.confirmed) {
    return {
      provider: 'outlook',
      action: 'send',
      confirmation_required: true
    };
  }

  return {
    provider: 'outlook',
    action: 'send',
    confirmation_required: false,
    message_id: buildMessageID(input)
  };
}

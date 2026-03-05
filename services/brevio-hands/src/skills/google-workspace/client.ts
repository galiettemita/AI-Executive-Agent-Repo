import { createHash } from 'node:crypto';

import type {
  GoogleWorkspaceCalendarEvent,
  GoogleWorkspaceDriveFile,
  GoogleWorkspaceInput,
  GoogleWorkspaceMail,
  GoogleWorkspaceOutput
} from './types.js';

const MAILS: GoogleWorkspaceMail[] = [
  {
    message_id: 'gmail_msg_001',
    from: 'ceo@example.com',
    subject: 'Board packet edits'
  },
  {
    message_id: 'gmail_msg_002',
    from: 'ops@example.com',
    subject: 'Weekly metrics summary'
  }
];

const EVENTS: GoogleWorkspaceCalendarEvent[] = [
  {
    event_id: 'gcal_evt_001',
    title: 'Leadership standup',
    start_time: '2026-03-05T16:00:00.000Z'
  },
  {
    event_id: 'gcal_evt_002',
    title: 'Investor prep',
    start_time: '2026-03-05T19:00:00.000Z'
  }
];

const FILES: GoogleWorkspaceDriveFile[] = [
  {
    file_id: 'gdrive_001',
    name: 'Q2-plan.docx',
    mime_type: 'application/vnd.openxmlformats-officedocument.wordprocessingml.document'
  },
  {
    file_id: 'gdrive_002',
    name: 'Finance-dashboard.xlsx',
    mime_type: 'application/vnd.openxmlformats-officedocument.spreadsheetml.sheet'
  }
];

function buildMessageID(input: GoogleWorkspaceInput): string {
  const digest = createHash('sha256')
    .update((input.to ?? []).join(','))
    .update('|')
    .update(input.subject ?? '')
    .update('|')
    .update((input.body ?? '').slice(0, 120))
    .digest('hex')
    .slice(0, 16);
  return `gmail_${digest}`;
}

export async function runClient(input: GoogleWorkspaceInput): Promise<GoogleWorkspaceOutput> {
  if (input.action === 'gmail_list') {
    return {
      provider: 'google-workspace',
      action: 'gmail_list',
      mails: MAILS
    };
  }

  if (input.action === 'calendar_list') {
    return {
      provider: 'google-workspace',
      action: 'calendar_list',
      events: EVENTS
    };
  }

  if (input.action === 'drive_search') {
    const query = input.query?.toLowerCase() ?? '';
    return {
      provider: 'google-workspace',
      action: 'drive_search',
      files: FILES.filter((file) => file.name.toLowerCase().includes(query))
    };
  }

  if (!input.to || !input.subject || !input.body) {
    throw new Error('GOOGLE_WORKSPACE_SEND_FIELDS_REQUIRED');
  }

  if (!input.confirmed) {
    return {
      provider: 'google-workspace',
      action: 'gmail_send',
      confirmation_required: true
    };
  }

  return {
    provider: 'google-workspace',
    action: 'gmail_send',
    confirmation_required: false,
    message_id: buildMessageID(input)
  };
}

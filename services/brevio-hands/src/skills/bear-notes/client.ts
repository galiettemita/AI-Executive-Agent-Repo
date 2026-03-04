import { createHash } from 'node:crypto';

import type {
  BEAR_NOTESInput,
  BEAR_NOTESNote,
  BEAR_NOTESOutput
} from './types.js';

const NOTES: BEAR_NOTESNote[] = [
  {
    note_id: 'bear-notes_001',
    title: 'Executive Daily Notes',
    content_preview: 'Top priorities and key blockers for the week.',
    updated_at: '2026-03-04T08:00:00.000Z'
  },
  {
    note_id: 'bear-notes_002',
    title: 'Project Memory',
    content_preview: 'Decisions, assumptions, and unresolved questions.',
    updated_at: '2026-03-04T10:30:00.000Z'
  }
];

function noteId(seed: string): string {
  return 'bear-notes_' + createHash('sha256').update(seed).digest('hex').slice(0, 8);
}

export async function runClient(input: BEAR_NOTESInput): Promise<BEAR_NOTESOutput> {
  if (input.action === 'list') {
    return {
      provider: 'bear-notes',
      action: 'list',
      notes: NOTES
    };
  }

  if (input.action === 'search') {
    const query = input.query?.toLowerCase() ?? '';
    return {
      provider: 'bear-notes',
      action: 'search',
      notes: NOTES.filter((note) => {
        const haystack = (note.title + ' ' + note.content_preview).toLowerCase();
        return haystack.includes(query);
      })
    };
  }

  if (input.action === 'create') {
    if (!input.title || !input.content) {
      throw new Error('BEAR_NOTES_CREATE_FIELDS_REQUIRED');
    }

    return {
      provider: 'bear-notes',
      action: 'create',
      note_id: noteId(input.title + '|' + input.content),
      notes: [
        {
          note_id: noteId(input.title + '|' + input.content),
          title: input.title,
          content_preview: input.content.slice(0, 80),
          updated_at: '2026-03-04T12:00:00.000Z'
        }
      ]
    };
  }

  if (!input.note_id || !input.content) {
    throw new Error('BEAR_NOTES_UPDATE_FIELDS_REQUIRED');
  }

  return {
    provider: 'bear-notes',
    action: 'update',
    note_id: input.note_id,
    notes: [
      {
        note_id: input.note_id,
        title: input.title ?? 'Updated Note',
        content_preview: input.content.slice(0, 80),
        updated_at: '2026-03-04T12:05:00.000Z'
      }
    ]
  };
}

import type { AppleNoteRecord, AppleNotesInput, AppleNotesOutput } from './types.js';

const NOTES: AppleNoteRecord[] = [
  {
    note_id: 'note_001',
    title: 'Board Meeting Prep',
    folder: 'Work',
    updated_at: '2026-03-04T14:20:00.000Z',
    preview: 'Finalize agenda, budget highlights, and open decisions.'
  },
  {
    note_id: 'note_002',
    title: 'Italy Trip Ideas',
    folder: 'Personal',
    updated_at: '2026-03-03T09:00:00.000Z',
    preview: 'Rome, Florence, and Amalfi itinerary ideas.'
  }
];

export async function runClient(input: AppleNotesInput): Promise<AppleNotesOutput> {
  if (input.action === 'create_note') {
    const created: AppleNoteRecord = {
      note_id: 'note_new_001',
      title: input.title ?? 'Untitled',
      folder: input.folder ?? 'Notes',
      updated_at: '2026-03-04T15:30:00.000Z',
      preview: (input.body ?? '').slice(0, 120)
    };

    return {
      provider: 'apple-notes',
      action: input.action,
      canonical_skill_id: 'apple-notes-skill',
      deprecated_alias: true,
      notes: [created],
      summary: `Created note "${created.title}" via deprecated alias route; canonical skill is apple-notes-skill.`
    };
  }

  if (input.action === 'search_notes') {
    const query = (input.query ?? '').toLowerCase();
    const notes = NOTES.filter(
      (note) =>
        note.title.toLowerCase().includes(query) || note.preview.toLowerCase().includes(query)
    );

    return {
      provider: 'apple-notes',
      action: input.action,
      canonical_skill_id: 'apple-notes-skill',
      deprecated_alias: true,
      notes,
      summary: `Found ${notes.length} note(s) for query "${input.query}".`
    };
  }

  return {
    provider: 'apple-notes',
    action: input.action,
    canonical_skill_id: 'apple-notes-skill',
    deprecated_alias: true,
    notes: NOTES,
    summary: `Returned ${NOTES.length} recent note(s) via alias adapter.`
  };
}

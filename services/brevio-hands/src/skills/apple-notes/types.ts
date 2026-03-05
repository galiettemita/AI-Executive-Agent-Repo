export type AppleNotesAction = 'create_note' | 'search_notes' | 'list_recent';

export interface AppleNotesInput {
  action: AppleNotesAction;
  title?: string;
  body?: string;
  folder?: string;
  query?: string;
}

export interface AppleNoteRecord {
  note_id: string;
  title: string;
  folder: string;
  updated_at: string;
  preview: string;
}

export interface AppleNotesOutput {
  provider: 'apple-notes';
  action: AppleNotesAction;
  canonical_skill_id: 'apple-notes-skill';
  deprecated_alias: true;
  notes: AppleNoteRecord[];
  summary: string;
}

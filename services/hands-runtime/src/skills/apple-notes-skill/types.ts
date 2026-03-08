export type APPLE_NOTES_SKILLAction = 'list' | 'create' | 'search' | 'update';

export interface APPLE_NOTES_SKILLInput {
  action: APPLE_NOTES_SKILLAction;
  note_id?: string;
  title?: string;
  content?: string;
  query?: string;
}

export interface APPLE_NOTES_SKILLNote {
  note_id: string;
  title: string;
  content_preview: string;
  updated_at: string;
}

export interface APPLE_NOTES_SKILLOutput {
  provider: 'apple-notes-skill';
  action: APPLE_NOTES_SKILLAction;
  note_id?: string;
  notes?: APPLE_NOTES_SKILLNote[];
}

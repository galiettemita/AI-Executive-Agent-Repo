export type OBSIDIANAction = 'list' | 'create' | 'search' | 'update';

export interface OBSIDIANInput {
  action: OBSIDIANAction;
  note_id?: string;
  title?: string;
  content?: string;
  query?: string;
}

export interface OBSIDIANNote {
  note_id: string;
  title: string;
  content_preview: string;
  updated_at: string;
}

export interface OBSIDIANOutput {
  provider: 'obsidian';
  action: OBSIDIANAction;
  note_id?: string;
  notes?: OBSIDIANNote[];
}

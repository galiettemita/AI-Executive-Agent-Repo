export type BEAR_NOTESAction = 'list' | 'create' | 'search' | 'update';

export interface BEAR_NOTESInput {
  action: BEAR_NOTESAction;
  note_id?: string;
  title?: string;
  content?: string;
  query?: string;
}

export interface BEAR_NOTESNote {
  note_id: string;
  title: string;
  content_preview: string;
  updated_at: string;
}

export interface BEAR_NOTESOutput {
  provider: 'bear-notes';
  action: BEAR_NOTESAction;
  note_id?: string;
  notes?: BEAR_NOTESNote[];
}

export type SECOND_BRAINAction = 'list' | 'create' | 'search' | 'update';

export interface SECOND_BRAINInput {
  action: SECOND_BRAINAction;
  note_id?: string;
  title?: string;
  content?: string;
  query?: string;
}

export interface SECOND_BRAINNote {
  note_id: string;
  title: string;
  content_preview: string;
  updated_at: string;
}

export interface SECOND_BRAINOutput {
  provider: 'second-brain';
  action: SECOND_BRAINAction;
  note_id?: string;
  notes?: SECOND_BRAINNote[];
}

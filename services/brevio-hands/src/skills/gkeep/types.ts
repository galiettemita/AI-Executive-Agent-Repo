export type GKEEPAction = 'list' | 'create' | 'search' | 'update';

export interface GKEEPInput {
  action: GKEEPAction;
  note_id?: string;
  title?: string;
  content?: string;
  query?: string;
}

export interface GKEEPNote {
  note_id: string;
  title: string;
  content_preview: string;
  updated_at: string;
}

export interface GKEEPOutput {
  provider: 'gkeep';
  action: GKEEPAction;
  note_id?: string;
  notes?: GKEEPNote[];
}

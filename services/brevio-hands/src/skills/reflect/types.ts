export type REFLECTAction = 'list' | 'create' | 'search' | 'update';

export interface REFLECTInput {
  action: REFLECTAction;
  note_id?: string;
  title?: string;
  content?: string;
  query?: string;
}

export interface REFLECTNote {
  note_id: string;
  title: string;
  content_preview: string;
  updated_at: string;
}

export interface REFLECTOutput {
  provider: 'reflect';
  action: REFLECTAction;
  note_id?: string;
  notes?: REFLECTNote[];
}

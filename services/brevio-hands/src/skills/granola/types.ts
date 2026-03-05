export type GranolaAction = 'summarize_note' | 'extract_actions';

export interface GranolaInput {
  action: GranolaAction;
  note_text?: string;
}

export interface GranolaOutput {
  provider: 'granola';
  action: GranolaAction;
  summary: string;
  action_items: string[];
  decisions: string[];
}

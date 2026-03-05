export type GammaAction = 'create_deck' | 'update_deck' | 'export_deck';

export interface GammaInput {
  action: GammaAction;
  topic?: string;
  deck_id?: string;
  slide_count?: number;
  format?: 'pdf' | 'pptx';
}

export interface GammaOutput {
  provider: 'gamma';
  action: GammaAction;
  deck_id: string;
  title: string;
  slide_count: number;
  export_url?: string;
  summary: string;
}

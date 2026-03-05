export type CraftAction = 'create_doc' | 'append_doc' | 'search_docs';

export interface CraftInput {
  action: CraftAction;
  doc_title?: string;
  doc_id?: string;
  content?: string;
  query?: string;
}

export interface CraftDoc {
  doc_id: string;
  title: string;
  updated_at: string;
}

export interface CraftOutput {
  provider: 'craft';
  action: CraftAction;
  docs: CraftDoc[];
  summary: string;
}

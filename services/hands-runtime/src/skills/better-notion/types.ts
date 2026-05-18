export type BetterNotionAction = 'create_page' | 'query_database' | 'update_page';

export interface BetterNotionInput {
  action: BetterNotionAction;
  page_title?: string;
  page_id?: string;
  database_id?: string;
  content?: string;
}

export interface BetterNotionPage {
  page_id: string;
  title: string;
  last_edited: string;
}

export interface BetterNotionOutput {
  provider: 'better-notion';
  action: BetterNotionAction;
  pages: BetterNotionPage[];
  summary: string;
}

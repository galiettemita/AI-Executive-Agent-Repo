export type NotionAction = 'search' | 'create_page' | 'append_block';

export interface NotionInput {
  action: NotionAction;
  query?: string;
  page_id?: string;
  title?: string;
  content?: string;
}

export interface NotionPage {
  page_id: string;
  title: string;
  last_edited_time: string;
}

export interface NotionOutput {
  provider: 'notion';
  action: NotionAction;
  page_id?: string;
  pages?: NotionPage[];
}

export type LiteratureReviewAction = 'search_papers';

export interface LiteratureReviewInput {
  action: LiteratureReviewAction;
  topic?: string;
}

export interface LiteraturePaper {
  title: string;
  year: number;
  venue: string;
  url: string;
}

export interface LiteratureReviewOutput {
  provider: 'literature-review';
  action: LiteratureReviewAction;
  papers: LiteraturePaper[];
  summary: string;
}

export interface ExaInput {
  query: string;
  max_results?: number;
  include_domains?: string[];
}

export interface ExaResultItem {
  title: string;
  url: string;
  snippet: string;
  score: number;
}

export interface ExaOutput {
  provider: 'exa';
  results: ExaResultItem[];
}

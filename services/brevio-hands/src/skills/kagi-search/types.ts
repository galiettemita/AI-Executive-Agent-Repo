export type KagiSearchAction = 'search';

export interface KagiSearchInput {
  action: KagiSearchAction;
  query?: string;
  max_results?: number;
}

export interface KagiResult {
  title: string;
  url: string;
  snippet: string;
}

export interface KagiSearchOutput {
  provider: 'kagi-search';
  action: KagiSearchAction;
  results: KagiResult[];
  summary: string;
}

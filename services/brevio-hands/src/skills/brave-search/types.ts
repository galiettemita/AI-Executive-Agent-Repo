export interface BraveSearchInput {
  query: string;
  max_results?: number;
}

export interface BraveSearchResult {
  title: string;
  url: string;
  description: string;
}

export interface BraveSearchOutput {
  provider: 'brave-search';
  results: BraveSearchResult[];
}

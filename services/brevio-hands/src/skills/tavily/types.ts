export interface TavilyInput {
  query: string;
  max_results?: number;
  include_domains?: string[];
}

export interface TavilyResult {
  title: string;
  url: string;
  content: string;
  score: number;
}

export interface TavilyOutput {
  results: TavilyResult[];
  provider: 'tavily';
}

export interface FirecrawlInput {
  query: string;
  max_results?: number;
}

export interface FirecrawlResult {
  title: string;
  url: string;
  content: string;
}

export interface FirecrawlOutput {
  provider: 'firecrawl';
  results: FirecrawlResult[];
}

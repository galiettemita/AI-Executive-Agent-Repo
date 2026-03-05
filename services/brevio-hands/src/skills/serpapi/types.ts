export interface SerpAPIInput {
  query: string;
  engine?: 'google' | 'amazon' | 'yelp';
  max_results?: number;
}

export interface SerpAPIResultItem {
  title: string;
  link: string;
  source: string;
}

export interface SerpAPIOutput {
  provider: 'serpapi';
  engine: 'google' | 'amazon' | 'yelp';
  results: SerpAPIResultItem[];
}

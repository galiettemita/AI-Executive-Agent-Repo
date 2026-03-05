export interface NewsAggregatorInput {
  topic?: string;
  max_items?: number;
}

export interface NewsAggregatorItem {
  source: string;
  title: string;
  url: string;
}

export interface NewsAggregatorOutput {
  provider: 'news-aggregator';
  items: NewsAggregatorItem[];
}

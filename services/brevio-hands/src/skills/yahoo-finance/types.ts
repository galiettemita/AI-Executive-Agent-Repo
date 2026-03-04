export interface YahooFinanceInput {
  action: 'quotes' | 'fundamentals' | 'news';
  symbols?: string[];
}

export interface YahooQuote {
  symbol: string;
  price: number;
  change_pct: number;
  volume: number;
}

export interface YahooFundamental {
  symbol: string;
  market_cap: number;
  pe_ratio?: number;
  dividend_yield_pct?: number;
}

export interface YahooFinanceNewsItem {
  headline: string;
  url: string;
  published_at: string;
}

export interface YahooFinanceOutput {
  provider: 'yahoo-finance';
  action: YahooFinanceInput['action'];
  quotes?: YahooQuote[];
  fundamentals?: YahooFundamental[];
  news?: YahooFinanceNewsItem[];
  disclaimer: string;
}

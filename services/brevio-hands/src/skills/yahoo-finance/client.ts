import type {
  YahooFundamental,
  YahooFinanceInput,
  YahooFinanceNewsItem,
  YahooFinanceOutput,
  YahooQuote
} from './types.js';

const QUOTES: Record<string, YahooQuote> = {
  AAPL: { symbol: 'AAPL', price: 214.35, change_pct: 0.84, volume: 53100000 },
  MSFT: { symbol: 'MSFT', price: 428.1, change_pct: -0.23, volume: 27200000 },
  TSLA: { symbol: 'TSLA', price: 186.72, change_pct: 1.45, volume: 91400000 }
};

const FUNDAMENTALS: Record<string, YahooFundamental> = {
  AAPL: { symbol: 'AAPL', market_cap: 3.2e12, pe_ratio: 31.2, dividend_yield_pct: 0.5 },
  MSFT: { symbol: 'MSFT', market_cap: 3.1e12, pe_ratio: 35.1, dividend_yield_pct: 0.7 },
  TSLA: { symbol: 'TSLA', market_cap: 0.6e12, pe_ratio: 61.4 }
};

const NEWS: YahooFinanceNewsItem[] = [
  {
    headline: 'Markets close mixed as tech leaders stabilize',
    url: 'https://finance.example.com/news/markets-close-mixed',
    published_at: '2026-03-04T16:00:00.000Z'
  },
  {
    headline: 'Analysts revise guidance for large-cap earnings',
    url: 'https://finance.example.com/news/large-cap-guidance',
    published_at: '2026-03-04T14:30:00.000Z'
  }
];

export async function runClient(input: YahooFinanceInput): Promise<YahooFinanceOutput> {
  if (input.action === 'quotes') {
    const quotes = (input.symbols ?? []).map((symbol) => QUOTES[symbol]).filter(Boolean);
    return {
      provider: 'yahoo-finance',
      action: 'quotes',
      quotes,
      disclaimer: 'Not financial advice.'
    };
  }

  if (input.action === 'fundamentals') {
    const fundamentals = (input.symbols ?? []).map((symbol) => FUNDAMENTALS[symbol]).filter(Boolean);
    return {
      provider: 'yahoo-finance',
      action: 'fundamentals',
      fundamentals,
      disclaimer: 'Not financial advice.'
    };
  }

  return {
    provider: 'yahoo-finance',
    action: 'news',
    news: NEWS,
    disclaimer: 'Not financial advice.'
  };
}

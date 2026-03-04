export interface FinancialMarketAnalysisInput {
  action: 'sentiment' | 'volatility' | 'correlation';
  symbols: string[];
}

export interface MarketMetric {
  symbol: string;
  score: number;
  summary: string;
}

export interface CorrelationMatrix {
  symbols: string[];
  matrix: number[][];
}

export interface FinancialMarketAnalysisOutput {
  provider: 'financial-market-analysis';
  action: FinancialMarketAnalysisInput['action'];
  metrics?: MarketMetric[];
  correlation?: CorrelationMatrix;
}

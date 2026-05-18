import type {
  CorrelationMatrix,
  FinancialMarketAnalysisInput,
  FinancialMarketAnalysisOutput,
  MarketMetric
} from './types.js';

function buildMetrics(symbols: string[], suffix: string): MarketMetric[] {
  return symbols.map((symbol, index) => ({
    symbol,
    score: Number((0.35 + index * 0.12).toFixed(2)),
    summary: `${symbol} ${suffix}`
  }));
}

function buildCorrelation(symbols: string[]): CorrelationMatrix {
  const matrix = symbols.map((_, i) =>
    symbols.map((__, j) => {
      if (i === j) {
        return 1;
      }
      return Number((0.2 + Math.abs(i - j) * 0.1).toFixed(2));
    })
  );
  return {
    symbols,
    matrix
  };
}

export async function runClient(
  input: FinancialMarketAnalysisInput
): Promise<FinancialMarketAnalysisOutput> {
  if (input.action === 'sentiment') {
    return {
      provider: 'financial-market-analysis',
      action: 'sentiment',
      metrics: buildMetrics(input.symbols, 'shows neutral-to-bullish sentiment')
    };
  }

  if (input.action === 'volatility') {
    return {
      provider: 'financial-market-analysis',
      action: 'volatility',
      metrics: buildMetrics(input.symbols, 'has moderate realized volatility')
    };
  }

  return {
    provider: 'financial-market-analysis',
    action: 'correlation',
    correlation: buildCorrelation(input.symbols)
  };
}

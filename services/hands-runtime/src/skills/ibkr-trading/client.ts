import type { IbkrTradingInput, IbkrTradingOutput } from './types.js';

export async function runClient(input: IbkrTradingInput): Promise<IbkrTradingOutput> {
  return {
    provider: 'ibkr-trading',
    action: input.action,
    symbol: input.symbol ?? 'AAPL',
    order_id: input.order_id ?? (input.action === 'place_order' ? 'ibkr-order-001' : undefined),
    status: input.action === 'quote_symbol' ? 'quoted' : input.action === 'place_order' ? 'submitted' : 'filled',
    price_usd: 189.42,
    summary: `IBKR action ${input.action} processed for ${input.symbol ?? 'AAPL'}.`
  };
}

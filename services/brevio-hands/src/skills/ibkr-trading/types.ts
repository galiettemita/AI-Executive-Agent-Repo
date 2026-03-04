export type IbkrTradingAction = 'quote_symbol' | 'place_order' | 'order_status';

export interface IbkrTradingInput {
  action: IbkrTradingAction;
  symbol?: string;
  side?: 'BUY' | 'SELL';
  quantity?: number;
  order_id?: string;
  confirmed?: boolean;
}

export interface IbkrTradingOutput {
  provider: 'ibkr-trading';
  action: IbkrTradingAction;
  symbol: string;
  order_id?: string;
  status: 'quoted' | 'submitted' | 'filled';
  price_usd: number;
  summary: string;
}

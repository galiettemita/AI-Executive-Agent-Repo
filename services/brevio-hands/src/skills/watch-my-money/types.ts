export type WatchMyMoneyAction = 'analyze_statement' | 'budget_alerts';

export interface WatchMyMoneyInput {
  action: WatchMyMoneyAction;
  monthly_income_cents?: number;
  transactions?: Array<{
    merchant: string;
    amount_cents: number;
    category: string;
  }>;
}

export interface WatchMyMoneyOutput {
  provider: 'watch-my-money';
  action: WatchMyMoneyAction;
  category_totals_cents: Record<string, number>;
  spend_rate_pct_of_income: number;
  alerts: string[];
  summary: string;
}

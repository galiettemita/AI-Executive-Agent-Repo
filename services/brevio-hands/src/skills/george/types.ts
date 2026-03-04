export type GeorgeAction = 'fetch_accounts' | 'analyze_transactions';

export interface GeorgeInput {
  action: GeorgeAction;
  account_id?: string;
}

export interface GeorgeAccount {
  account_id: string;
  balance_eur: number;
  currency: string;
}

export interface GeorgeOutput {
  provider: 'george';
  action: GeorgeAction;
  accounts: GeorgeAccount[];
  summary: string;
}

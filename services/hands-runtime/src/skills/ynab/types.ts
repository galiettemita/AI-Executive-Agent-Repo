export type YNABAction = 'summary' | 'accounts' | 'transactions';

export interface YNABInput {
  action: YNABAction;
  budget_id?: string;
  account_id?: string;
}

export interface YNABAccount {
  account_id: string;
  name: string;
  balance_cents: number;
}

export interface YNABTransaction {
  transaction_id: string;
  account_id: string;
  payee: string;
  amount_cents: number;
  date: string;
}

export interface YNABOutput {
  provider: 'ynab';
  action: YNABAction;
  budget_id: string;
  total_budget_cents?: number;
  accounts?: YNABAccount[];
  transactions?: YNABTransaction[];
}

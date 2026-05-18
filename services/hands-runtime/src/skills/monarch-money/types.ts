export interface MonarchMoneyInput {
  action: 'accounts' | 'transactions' | 'budgets';
  account_id?: string;
  month?: string;
}

export interface MonarchAccount {
  account_id: string;
  name: string;
  balance_cents: number;
}

export interface MonarchTransaction {
  transaction_id: string;
  account_id: string;
  merchant: string;
  amount_cents: number;
  category: string;
  posted_at: string;
}

export interface MonarchBudget {
  category: string;
  budget_cents: number;
  spent_cents: number;
}

export interface MonarchMoneyOutput {
  provider: 'monarch-money';
  action: MonarchMoneyInput['action'];
  accounts?: MonarchAccount[];
  transactions?: MonarchTransaction[];
  budgets?: MonarchBudget[];
}

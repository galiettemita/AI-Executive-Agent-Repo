export interface CopilotMoneyInput {
  action: 'accounts' | 'transactions' | 'net_worth';
  account_id?: string;
  from_date?: string;
  to_date?: string;
}

export interface CopilotMoneyAccount {
  account_id: string;
  name: string;
  balance_cents: number;
  account_type: 'checking' | 'savings' | 'credit';
}

export interface CopilotMoneyTransaction {
  transaction_id: string;
  account_id: string;
  merchant: string;
  amount_cents: number;
  category: string;
  posted_at: string;
}

export interface CopilotMoneyOutput {
  provider: 'copilot-money';
  action: CopilotMoneyInput['action'];
  accounts?: CopilotMoneyAccount[];
  transactions?: CopilotMoneyTransaction[];
  net_worth_cents?: number;
}

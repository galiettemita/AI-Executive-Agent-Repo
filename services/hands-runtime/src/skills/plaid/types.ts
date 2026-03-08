export type PlaidAction = 'accounts' | 'transactions' | 'balance';

export interface PlaidInput {
  action: PlaidAction;
  account_id?: string;
}

export interface PlaidAccount {
  account_id: string;
  name: string;
  mask: string;
  subtype: string;
}

export interface PlaidTransaction {
  transaction_id: string;
  account_id: string;
  name: string;
  amount: number;
  date: string;
}

export interface PlaidBalance {
  account_id: string;
  available: number;
  current: number;
}

export interface PlaidOutput {
  provider: 'plaid';
  action: PlaidAction;
  accounts?: PlaidAccount[];
  transactions?: PlaidTransaction[];
  balances?: PlaidBalance[];
}

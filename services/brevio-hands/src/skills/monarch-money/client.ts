import type {
  MonarchAccount,
  MonarchBudget,
  MonarchMoneyInput,
  MonarchMoneyOutput,
  MonarchTransaction
} from './types.js';

const ACCOUNTS: MonarchAccount[] = [
  {
    account_id: 'mon_acc_001',
    name: 'Main Checking',
    balance_cents: 923400
  },
  {
    account_id: 'mon_acc_002',
    name: 'Travel Card',
    balance_cents: -124500
  }
];

const TRANSACTIONS: MonarchTransaction[] = [
  {
    transaction_id: 'mon_tx_001',
    account_id: 'mon_acc_001',
    merchant: 'Downtown Garage',
    amount_cents: -1800,
    category: 'Parking',
    posted_at: '2026-03-04T15:00:00.000Z'
  },
  {
    transaction_id: 'mon_tx_002',
    account_id: 'mon_acc_001',
    merchant: 'Executive Cafe',
    amount_cents: -2850,
    category: 'Meals',
    posted_at: '2026-03-03T12:10:00.000Z'
  }
];

const BUDGETS: MonarchBudget[] = [
  {
    category: 'Meals',
    budget_cents: 120000,
    spent_cents: 78200
  },
  {
    category: 'Transport',
    budget_cents: 80000,
    spent_cents: 31400
  }
];

export async function runClient(input: MonarchMoneyInput): Promise<MonarchMoneyOutput> {
  if (input.action === 'accounts') {
    return {
      provider: 'monarch-money',
      action: 'accounts',
      accounts: ACCOUNTS
    };
  }

  if (input.action === 'transactions') {
    return {
      provider: 'monarch-money',
      action: 'transactions',
      transactions: TRANSACTIONS.filter((item) => item.account_id === input.account_id)
    };
  }

  return {
    provider: 'monarch-money',
    action: 'budgets',
    budgets: BUDGETS
  };
}

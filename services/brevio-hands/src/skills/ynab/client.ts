import type { YNABAccount, YNABInput, YNABOutput, YNABTransaction } from './types.js';

const ACCOUNTS: YNABAccount[] = [
  {
    account_id: 'acct_checking',
    name: 'Checking',
    balance_cents: 524030
  },
  {
    account_id: 'acct_savings',
    name: 'Savings',
    balance_cents: 1834500
  }
];

const TRANSACTIONS: YNABTransaction[] = [
  {
    transaction_id: 'txn_001',
    account_id: 'acct_checking',
    payee: 'Coffee Bar',
    amount_cents: -620,
    date: '2026-03-01'
  },
  {
    transaction_id: 'txn_002',
    account_id: 'acct_checking',
    payee: 'Office Supplies',
    amount_cents: -4825,
    date: '2026-03-02'
  },
  {
    transaction_id: 'txn_003',
    account_id: 'acct_savings',
    payee: 'Interest',
    amount_cents: 315,
    date: '2026-03-02'
  }
];

export async function runClient(input: YNABInput): Promise<YNABOutput> {
  const budgetId = input.budget_id ?? 'primary-budget';

  if (input.action === 'summary') {
    const totalBudgetCents = ACCOUNTS.reduce((sum, account) => sum + account.balance_cents, 0);
    return {
      provider: 'ynab',
      action: 'summary',
      budget_id: budgetId,
      total_budget_cents: totalBudgetCents
    };
  }

  if (input.action === 'accounts') {
    return {
      provider: 'ynab',
      action: 'accounts',
      budget_id: budgetId,
      accounts: ACCOUNTS
    };
  }

  if (input.account_id && !ACCOUNTS.some((account) => account.account_id === input.account_id)) {
    throw new Error('YNAB_ACCOUNT_NOT_FOUND');
  }

  return {
    provider: 'ynab',
    action: 'transactions',
    budget_id: budgetId,
    transactions: TRANSACTIONS.filter((txn) => !input.account_id || txn.account_id === input.account_id)
  };
}

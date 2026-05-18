import type {
  CopilotMoneyAccount,
  CopilotMoneyInput,
  CopilotMoneyOutput,
  CopilotMoneyTransaction
} from './types.js';

const ACCOUNTS: CopilotMoneyAccount[] = [
  {
    account_id: 'cp_chk_001',
    name: 'Operating Checking',
    balance_cents: 1250043,
    account_type: 'checking'
  },
  {
    account_id: 'cp_svg_010',
    name: 'Emergency Savings',
    balance_cents: 8430021,
    account_type: 'savings'
  }
];

const TRANSACTIONS: CopilotMoneyTransaction[] = [
  {
    transaction_id: 'cp_tx_1001',
    account_id: 'cp_chk_001',
    merchant: 'Boardroom Bistro',
    amount_cents: -7425,
    category: 'Dining',
    posted_at: '2026-03-03T14:20:00.000Z'
  },
  {
    transaction_id: 'cp_tx_1002',
    account_id: 'cp_chk_001',
    merchant: 'City Mobility',
    amount_cents: -2150,
    category: 'Transport',
    posted_at: '2026-03-04T08:10:00.000Z'
  }
];

export async function runClient(input: CopilotMoneyInput): Promise<CopilotMoneyOutput> {
  if (input.action === 'accounts') {
    return {
      provider: 'copilot-money',
      action: 'accounts',
      accounts: ACCOUNTS
    };
  }

  if (input.action === 'transactions') {
    return {
      provider: 'copilot-money',
      action: 'transactions',
      transactions: TRANSACTIONS.filter((tx) => tx.account_id === input.account_id)
    };
  }

  const netWorthCents = ACCOUNTS.reduce((sum, account) => sum + account.balance_cents, 0);
  return {
    provider: 'copilot-money',
    action: 'net_worth',
    net_worth_cents: netWorthCents
  };
}

import type {
  PlaidAccount,
  PlaidBalance,
  PlaidInput,
  PlaidOutput,
  PlaidTransaction
} from './types.js';

const ACCOUNTS: PlaidAccount[] = [
  {
    account_id: 'plaid_checking',
    name: 'Main Checking',
    mask: '1234',
    subtype: 'checking'
  },
  {
    account_id: 'plaid_savings',
    name: 'Reserve Savings',
    mask: '9876',
    subtype: 'savings'
  }
];

const TRANSACTIONS: PlaidTransaction[] = [
  {
    transaction_id: 'ptxn_001',
    account_id: 'plaid_checking',
    name: 'Team Lunch',
    amount: 48.5,
    date: '2026-03-02'
  },
  {
    transaction_id: 'ptxn_002',
    account_id: 'plaid_checking',
    name: 'Rideshare',
    amount: 17.2,
    date: '2026-03-03'
  }
];

const BALANCES: PlaidBalance[] = [
  {
    account_id: 'plaid_checking',
    available: 2410.2,
    current: 2431.2
  },
  {
    account_id: 'plaid_savings',
    available: 11840.55,
    current: 11840.55
  }
];

function assertAccountExists(accountID?: string): void {
  if (!accountID) {
    return;
  }
  if (!ACCOUNTS.some((account) => account.account_id === accountID)) {
    throw new Error('PLAID_ACCOUNT_NOT_FOUND');
  }
}

export async function runClient(input: PlaidInput): Promise<PlaidOutput> {
  assertAccountExists(input.account_id);

  if (input.action === 'accounts') {
    return {
      provider: 'plaid',
      action: 'accounts',
      accounts: ACCOUNTS
    };
  }

  if (input.action === 'transactions') {
    return {
      provider: 'plaid',
      action: 'transactions',
      transactions: TRANSACTIONS.filter(
        (transaction) => !input.account_id || transaction.account_id === input.account_id
      )
    };
  }

  return {
    provider: 'plaid',
    action: 'balance',
    balances: BALANCES.filter((balance) => !input.account_id || balance.account_id === input.account_id)
  };
}

// Plan §6 step 3 — Real YNAB API

interface SkillContext { token?: string; user_id?: string; }

function isoDate(d: Date): string { return d.toISOString().slice(0, 10); }
function daysAgo(n: number): string {
  const d = new Date();
  d.setDate(d.getDate() - n);
  return isoDate(d);
}

const YNAB_BASE = 'https://api.youneedabudget.com/v1';

export async function runClient(
  input: Record<string, any>,
  ctx?: SkillContext
): Promise<any> {
  const token = process.env.YNAB_TOKEN;
  if (!token) throw new Error('YNAB_TOKEN env var is required');

  const budgetId = input.budget_id ?? 'last-used';

  async function ynabGet(path: string): Promise<any> {
    const res = await fetch(`${YNAB_BASE}${path}`, {
      headers: { authorization: `Bearer ${token}` },
    });
    const body = await res.json();
    if (!res.ok) throw new Error(`YNAB API error ${res.status}: ${JSON.stringify(body)}`);
    return body;
  }

  switch (input.action) {
    case 'summary': {
      const body = await ynabGet(`/budgets/${budgetId}`);
      return { budget: body.data.budget };
    }
    case 'accounts': {
      const body = await ynabGet(`/budgets/${budgetId}/accounts`);
      return { accounts: body.data.accounts.filter((a: any) => !a.closed) };
    }
    case 'transactions': {
      const since_date = daysAgo(30);
      const body = await ynabGet(`/budgets/${budgetId}/transactions?since_date=${since_date}`);
      return { transactions: body.data.transactions };
    }
    default:
      throw new Error(`Unknown YNAB action: ${input.action}`);
  }
}

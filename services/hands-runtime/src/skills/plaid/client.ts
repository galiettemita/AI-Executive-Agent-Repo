// Plan §6 step 2 — Real Plaid API
// ctx.token for per-user access tokens; PLAID_ENV controls base URL

interface SkillContext { token?: string; user_id?: string; }

function isoDate(d: Date): string { return d.toISOString().slice(0, 10); }
function today(): string { return isoDate(new Date()); }
function daysAgo(n: number): string {
  const d = new Date();
  d.setDate(d.getDate() - n);
  return isoDate(d);
}

const ENV_URL: Record<string, string> = {
  sandbox:     'https://sandbox.plaid.com',
  development: 'https://development.plaid.com',
  production:  'https://production.plaid.com',
};

export async function runClient(
  input: Record<string, any>,
  ctx?: SkillContext
): Promise<any> {
  const baseUrl   = ENV_URL[process.env.PLAID_ENV ?? 'sandbox'] ?? 'https://sandbox.plaid.com';
  const client_id = process.env.PLAID_CLIENT_ID;
  const secret    = process.env.PLAID_SECRET;
  if (!client_id || !secret) {
    throw new Error('PLAID_CLIENT_ID and PLAID_SECRET env vars are required');
  }

  const access_token = ctx?.token ?? process.env.PLAID_ACCESS_TOKEN;
  if (!access_token) {
    throw new Error('Plaid access_token required: provide ctx.token or set PLAID_ACCESS_TOKEN');
  }

  async function plaidPost(path: string, extra: Record<string, any> = {}): Promise<any> {
    const res = await fetch(`${baseUrl}${path}`, {
      method:  'POST',
      headers: { 'content-type': 'application/json' },
      body:    JSON.stringify({ client_id, secret, access_token, ...extra }),
    });
    const body = await res.json();
    if (!res.ok) throw new Error(`Plaid API error ${res.status}: ${JSON.stringify(body)}`);
    return body;
  }

  switch (input.action) {
    case 'accounts': {
      const body = await plaidPost('/accounts/get');
      return { accounts: body.accounts };
    }
    case 'transactions': {
      const body = await plaidPost('/transactions/get', {
        start_date: input.start_date ?? daysAgo(30),
        end_date:   input.end_date ?? today(),
        options:    { count: 50 },
      });
      return { transactions: body.transactions, total_transactions: body.total_transactions };
    }
    case 'balance': {
      const body = await plaidPost('/accounts/balance/get');
      return { accounts: body.accounts };
    }
    default:
      throw new Error(`Unknown Plaid action: ${input.action}`);
  }
}

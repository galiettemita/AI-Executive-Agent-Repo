// Plan §5 — Real Monarch Money GraphQL API
// ctx.token → MONARCH_TOKEN fallback

interface SkillContext { token?: string; user_id?: string; }

const MONARCH_GQL = 'https://api.monarchmoney.com/graphql';

export async function runClient(
  input: Record<string, any>,
  ctx?: SkillContext
): Promise<any> {
  const token = ctx?.token ?? process.env.MONARCH_TOKEN;
  if (!token) throw new Error('Monarch Money token required: provide ctx.token or set MONARCH_TOKEN');

  async function gql(query: string): Promise<any> {
    const res = await fetch(MONARCH_GQL, {
      method:  'POST',
      headers: {
        authorization:  `Token ${token}`,
        'content-type': 'application/json',
      },
      body: JSON.stringify({ query }),
    });
    if (!res.ok) throw new Error(`Monarch Money HTTP error: ${res.status}`);
    const body = await res.json();
    if (body.errors?.length) throw new Error(`Monarch Money GraphQL error: ${body.errors[0].message}`);
    return body.data;
  }

  switch (input.action) {

    case 'accounts': {
      const data = await gql(`{
        accounts {
          id name type subtype currentBalance
          institution { name }
          isHidden
        }
      }`);
      return { accounts: data.accounts.filter((a: any) => !a.isHidden) };
    }

    case 'transactions': {
      const data = await gql(`{
        transactions(limit: 50) {
          edges {
            node { id date merchant { name } amount category { name } notes }
          }
        }
      }`);
      return { transactions: data.transactions.edges.map((e: any) => e.node) };
    }

    case 'budgets': {
      const data = await gql(`{
        budgets {
          category { name }
          budgeted
          actual
          remaining
        }
      }`);
      return { budgets: data.budgets };
    }

    default:
      throw new Error(`Unknown Monarch Money action: ${input.action}. Valid: accounts, transactions, budgets`);
  }
}

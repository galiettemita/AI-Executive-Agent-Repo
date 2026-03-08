import type { WatchMyMoneyInput, WatchMyMoneyOutput } from './types.js';

export async function runClient(input: WatchMyMoneyInput): Promise<WatchMyMoneyOutput> {
  const transactions = input.transactions ?? [];
  const categoryTotals: Record<string, number> = {};
  let total = 0;

  for (const transaction of transactions) {
    categoryTotals[transaction.category] = (categoryTotals[transaction.category] ?? 0) + transaction.amount_cents;
    total += transaction.amount_cents;
  }

  const income = input.monthly_income_cents ?? 1;
  const spendRate = Number(((total / income) * 100).toFixed(2));

  const alerts: string[] = [];
  if (spendRate > 85) {
    alerts.push('Spend exceeds 85% of monthly income target.');
  }
  if ((categoryTotals['Dining'] ?? 0) > 20000) {
    alerts.push('Dining category exceeded $200 monthly threshold.');
  }

  return {
    provider: 'watch-my-money',
    action: input.action,
    category_totals_cents: categoryTotals,
    spend_rate_pct_of_income: spendRate,
    alerts,
    summary: `Analyzed ${transactions.length} transaction(s) with spend rate ${spendRate}% of monthly income.`
  };
}

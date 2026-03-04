import type {
  SmartExpenseEntry,
  SmartExpenseTrackerInput,
  SmartExpenseTrackerOutput
} from './types.js';

const BASE_ENTRIES: SmartExpenseEntry[] = [
  {
    entry_id: 'exp_001',
    merchant: 'Boardroom Bistro',
    amount_cents: 7425,
    category: 'Dining',
    occurred_on: '2026-03-04'
  },
  {
    entry_id: 'exp_002',
    merchant: 'City Mobility',
    amount_cents: 2150,
    category: 'Transport',
    occurred_on: '2026-03-04'
  }
];

function summarize(entries: SmartExpenseEntry[]): Pick<SmartExpenseTrackerOutput, 'today_spend_cents' | 'month_spend_cents' | 'budget_alerts'> {
  const todaySpend = entries.filter((entry) => entry.occurred_on === '2026-03-04').reduce((sum, entry) => sum + entry.amount_cents, 0);
  const monthSpend = entries.reduce((sum, entry) => sum + entry.amount_cents, 0);

  const alerts: string[] = [];
  if (monthSpend > 50000) {
    alerts.push('Monthly discretionary spend exceeded $500 threshold.');
  }
  if (todaySpend > 20000) {
    alerts.push('Today spending pace is above daily target.');
  }

  return {
    today_spend_cents: todaySpend,
    month_spend_cents: monthSpend,
    budget_alerts: alerts
  };
}

export async function runClient(input: SmartExpenseTrackerInput): Promise<SmartExpenseTrackerOutput> {
  const entries = [...BASE_ENTRIES];

  if (input.action === 'log_expense') {
    entries.unshift({
      entry_id: 'exp_new_001',
      merchant: input.merchant ?? 'Unknown',
      amount_cents: input.amount_cents ?? 0,
      category: input.category ?? 'Uncategorized',
      occurred_on: input.occurred_on ?? '2026-03-04'
    });
  }

  const summaryMetrics = summarize(entries);

  return {
    provider: 'smart-expense-tracker',
    action: input.action,
    entries,
    ...summaryMetrics,
    summary: `Tracked ${entries.length} expense item(s). Today: $${(summaryMetrics.today_spend_cents / 100).toFixed(2)}, month: $${(summaryMetrics.month_spend_cents / 100).toFixed(2)}.`
  };
}

import type { ExpenseTrackerProInput, ExpenseTrackerProOutput } from './types.js';

const BASE = [
  {
    entry_id: 'etp_001',
    merchant: 'Metro Grocer',
    amount_cents: 18450,
    category: 'Groceries',
    occurred_on: '2026-03-03'
  },
  {
    entry_id: 'etp_002',
    merchant: 'Fuel Station 8',
    amount_cents: 6200,
    category: 'Transport',
    occurred_on: '2026-03-02'
  }
];

function aggregate(entries: ExpenseTrackerProOutput['entries']): {
  totals_by_category: Record<string, number>;
  total_cents: number;
} {
  const totals: Record<string, number> = {};
  let total = 0;
  for (const entry of entries) {
    totals[entry.category] = (totals[entry.category] ?? 0) + entry.amount_cents;
    total += entry.amount_cents;
  }
  return { totals_by_category: totals, total_cents: total };
}

export async function runClient(input: ExpenseTrackerProInput): Promise<ExpenseTrackerProOutput> {
  const entries = [...BASE];
  if (input.action === 'add_expense') {
    entries.unshift({
      entry_id: 'etp_new_001',
      merchant: input.merchant ?? 'Unknown',
      amount_cents: input.amount_cents ?? 0,
      category: input.category ?? 'Uncategorized',
      occurred_on: input.occurred_on ?? '2026-03-04'
    });
  }

  const aggregated = aggregate(entries);

  return {
    provider: 'expense-tracker-pro',
    action: input.action,
    entries,
    ...aggregated,
    summary: `Tracked ${entries.length} entries with total spend $${(aggregated.total_cents / 100).toFixed(2)}.`
  };
}

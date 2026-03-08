export type ExpenseTrackerProAction = 'add_expense' | 'monthly_summary' | 'category_breakdown';

export interface ExpenseTrackerProInput {
  action: ExpenseTrackerProAction;
  merchant?: string;
  amount_cents?: number;
  category?: string;
  occurred_on?: string;
  month?: string;
}

export interface ExpenseTrackerProOutput {
  provider: 'expense-tracker-pro';
  action: ExpenseTrackerProAction;
  entries: Array<{
    entry_id: string;
    merchant: string;
    amount_cents: number;
    category: string;
    occurred_on: string;
  }>;
  totals_by_category: Record<string, number>;
  total_cents: number;
  summary: string;
}

export type SmartExpenseTrackerAction = 'log_expense' | 'daily_briefing' | 'budget_status';

export interface SmartExpenseTrackerInput {
  action: SmartExpenseTrackerAction;
  merchant?: string;
  amount_cents?: number;
  category?: string;
  occurred_on?: string;
  note?: string;
}

export interface SmartExpenseEntry {
  entry_id: string;
  merchant: string;
  amount_cents: number;
  category: string;
  occurred_on: string;
}

export interface SmartExpenseTrackerOutput {
  provider: 'smart-expense-tracker';
  action: SmartExpenseTrackerAction;
  entries: SmartExpenseEntry[];
  today_spend_cents: number;
  month_spend_cents: number;
  budget_alerts: string[];
  summary: string;
}

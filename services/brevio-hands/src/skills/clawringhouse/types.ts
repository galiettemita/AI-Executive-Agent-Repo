export type ClawringhouseAction =
  | 'detect_reorder_need'
  | 'proactive_recommendations'
  | 'schedule_reorder_reminder';

export interface HouseholdItem {
  name: string;
  days_since_last_order: number;
  typical_cycle_days: number;
  estimated_units_left: number;
}

export interface ClawringhouseInput {
  action: ClawringhouseAction;
  household_items?: HouseholdItem[];
  reminder_time_local?: string;
}

export interface ClawringhouseOutput {
  provider: 'clawringhouse';
  action: ClawringhouseAction;
  recommendations: Array<{
    item: string;
    urgency: 'low' | 'medium' | 'high';
    reason: string;
  }>;
  next_reminder_local?: string;
  summary: string;
}

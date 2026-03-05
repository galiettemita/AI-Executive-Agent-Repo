export type RefundRadarAction = 'scan_recurring_charges' | 'draft_refund_request';

export interface RefundRadarInput {
  action: RefundRadarAction;
  merchant?: string;
  amount_cents?: number;
  reason?: string;
}

export interface RefundFlaggedCharge {
  merchant: string;
  amount_cents: number;
  frequency: 'weekly' | 'monthly' | 'annual';
  confidence: number;
}

export interface RefundRadarOutput {
  provider: 'refund-radar';
  action: RefundRadarAction;
  flagged_charges: RefundFlaggedCharge[];
  draft_message?: string;
  summary: string;
}

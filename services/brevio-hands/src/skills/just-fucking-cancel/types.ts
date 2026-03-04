export type JustFuckingCancelAction = 'scan_subscriptions' | 'draft_cancellation';

export interface JustFuckingCancelInput {
  action: JustFuckingCancelAction;
  transactions_csv?: string;
  merchant_name?: string;
}

export interface SubscriptionFinding {
  merchant: string;
  amount_usd: number;
  cadence: string;
}

export interface JustFuckingCancelOutput {
  provider: 'just-fucking-cancel';
  action: JustFuckingCancelAction;
  findings: SubscriptionFinding[];
  draft_message?: string;
  summary: string;
}

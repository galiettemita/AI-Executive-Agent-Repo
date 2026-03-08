import type { RefundRadarInput, RefundRadarOutput } from './types.js';

const FLAGGED: RefundRadarOutput['flagged_charges'] = [
  { merchant: 'StreamPlus', amount_cents: 1599, frequency: 'monthly', confidence: 0.93 },
  { merchant: 'CloudStorageX', amount_cents: 999, frequency: 'monthly', confidence: 0.88 }
];

export async function runClient(input: RefundRadarInput): Promise<RefundRadarOutput> {
  if (input.action === 'draft_refund_request') {
    return {
      provider: 'refund-radar',
      action: input.action,
      flagged_charges: FLAGGED,
      draft_message: `Hello ${input.merchant} support,\n\nI am requesting a refund for a recent charge of $${(
        (input.amount_cents ?? 0) / 100
      ).toFixed(2)}. ${input.reason ?? 'This charge was unexpected and I did not intend to renew.'}\n\nThank you.`,
      summary: `Prepared refund request draft for ${input.merchant}.`
    };
  }

  return {
    provider: 'refund-radar',
    action: input.action,
    flagged_charges: FLAGGED,
    summary: `Detected ${FLAGGED.length} potentially refundable recurring charge(s).`
  };
}

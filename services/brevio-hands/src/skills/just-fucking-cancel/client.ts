import type { JustFuckingCancelInput, JustFuckingCancelOutput } from './types.js';

export async function runClient(input: JustFuckingCancelInput): Promise<JustFuckingCancelOutput> {
  const findings = [
    {
      merchant: input.merchant_name ?? 'Streaming Service Pro',
      amount_usd: 19.99,
      cadence: 'monthly'
    }
  ];

  return {
    provider: 'just-fucking-cancel',
    action: input.action,
    findings,
    draft_message:
      input.action === 'draft_cancellation'
        ? `Hello, please cancel my subscription for ${input.merchant_name} effective immediately.`
        : undefined,
    summary: `Generated ${findings.length} cancellation candidate(s).`
  };
}

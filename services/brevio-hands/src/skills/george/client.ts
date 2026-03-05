import type { GeorgeInput, GeorgeOutput } from './types.js';

export async function runClient(input: GeorgeInput): Promise<GeorgeOutput> {
  const accounts = [
    {
      account_id: input.account_id ?? 'george-main',
      balance_eur: 12450.35,
      currency: 'EUR' as const
    }
  ];

  return {
    provider: 'george',
    action: input.action,
    accounts,
    summary: `George action ${input.action} returned ${accounts.length} account snapshot(s).`
  };
}

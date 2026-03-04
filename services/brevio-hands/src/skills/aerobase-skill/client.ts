import type { AerobaseInput, AerobaseOutput } from './types.js';

export async function runClient(input: AerobaseInput): Promise<AerobaseOutput> {
  const itineraries = [
    {
      flight_number: 'AB123',
      duration_minutes: 415,
      price_usd: 489,
      jetlag_score: 32
    },
    {
      flight_number: 'AB452',
      duration_minutes: 460,
      price_usd: 429,
      jetlag_score: 40
    }
  ];

  return {
    provider: 'aerobase-skill',
    action: input.action,
    itineraries,
    summary: `Compared ${itineraries.length} itineraries from ${input.origin} to ${input.destination}.`
  };
}

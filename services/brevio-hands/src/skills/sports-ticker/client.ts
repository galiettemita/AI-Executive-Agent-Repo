import type { SportsTickerInput, SportsTickerOutput } from './types.js';

export async function runClient(input: SportsTickerInput): Promise<SportsTickerOutput> {
  const items = [
    {
      title: `${input.team ?? 'Team'} vs Rival`,
      status: input.action === 'get_score' ? 'Final: 112-108' : 'Scheduled: 7:30 PM local'
    }
  ];

  return {
    provider: 'sports-ticker',
    action: input.action,
    items,
    summary: `Returned ${input.action} update for ${input.league}.`
  };
}

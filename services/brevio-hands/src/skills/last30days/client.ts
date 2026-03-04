import type { Last30DaysInput, Last30DaysOutput } from './types.js';

export async function runClient(input: Last30DaysInput): Promise<Last30DaysOutput> {
  return {
    provider: 'last30days',
    action: 'scan_topic',
    highlights: [`Recent discussion trend for ${input.query}`],
    sources: ['https://news.example.com/topic-1'],
    summary: `Compiled cross-platform trend highlights for ${input.query}.`
  };
}

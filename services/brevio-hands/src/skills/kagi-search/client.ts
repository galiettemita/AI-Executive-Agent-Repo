import type { KagiSearchInput, KagiSearchOutput } from './types.js';

export async function runClient(input: KagiSearchInput): Promise<KagiSearchOutput> {
  const results = [
    {
      title: `Result for ${input.query}`,
      url: 'https://search.example.com/result-1',
      snippet: 'Deterministic Kagi-like result snippet for testing.'
    }
  ];

  return {
    provider: 'kagi-search',
    action: 'search',
    results,
    summary: `Returned ${results.length} results for query "${input.query}".`
  };
}

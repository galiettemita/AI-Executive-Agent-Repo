import type { PerplexityInput, PerplexityOutput } from './types.js';

export async function runClient(input: PerplexityInput): Promise<PerplexityOutput> {
  const model = input.model ?? 'sonar-medium-online';
  return {
    provider: 'perplexity',
    answer: `Model ${model} summary for query: ${input.query}`,
    citations: [
      {
        title: 'Executive systems research brief',
        url: 'https://briefs.example.com/executive-systems'
      },
      {
        title: 'Operational cadence benchmarks',
        url: 'https://benchmarks.example.com/operational-cadence'
      }
    ]
  };
}

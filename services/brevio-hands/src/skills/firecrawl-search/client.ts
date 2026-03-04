import type { FirecrawlInput, FirecrawlOutput, FirecrawlResult } from './types.js';

const RESULTS: FirecrawlResult[] = [
  {
    title: 'Executive assistant benchmark report',
    url: 'https://crawl.example.com/exec-assistant-benchmark',
    content: 'Benchmark data comparing assistant response quality and latency.'
  },
  {
    title: 'Operational dashboard design patterns',
    url: 'https://crawl.example.com/ops-dashboard-patterns',
    content: 'Patterns for dashboard information hierarchy and alerting.'
  },
  {
    title: 'Leadership meeting digest strategy',
    url: 'https://crawl.example.com/leadership-meeting-digest',
    content: 'How to distill long meetings into concise executive digests.'
  }
];

export async function runClient(input: FirecrawlInput): Promise<FirecrawlOutput> {
  const terms = input.query.toLowerCase().split(/\s+/).filter((term) => term.length > 1);
  const results = RESULTS.filter((result) => {
    const haystack = `${result.title} ${result.content}`.toLowerCase();
    return terms.some((term) => haystack.includes(term));
  }).slice(0, input.max_results ?? 5);

  return {
    provider: 'firecrawl',
    results
  };
}

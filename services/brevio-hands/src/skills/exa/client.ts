import type { ExaInput, ExaOutput, ExaResultItem } from './types.js';

const RESULTS: ExaResultItem[] = [
  {
    title: 'Executive operating cadence templates',
    url: 'https://insights.example.com/executive-cadence',
    snippet: 'Structured weekly cadence frameworks for leadership teams.',
    score: 0.94
  },
  {
    title: 'Strategic planning with AI copilots',
    url: 'https://research.example.com/ai-strategy-copilots',
    snippet: 'How AI copilots improve strategic planning consistency.',
    score: 0.91
  },
  {
    title: 'Decision brief design guide',
    url: 'https://docs.example.com/decision-brief-guide',
    snippet: 'A practical guide to concise decision briefs for executives.',
    score: 0.87
  }
];

export async function runClient(input: ExaInput): Promise<ExaOutput> {
  const terms = input.query
    .toLowerCase()
    .split(/\s+/)
    .map((term) => term.trim())
    .filter((term) => term.length > 1);

  const filtered = RESULTS.filter((item) => {
    const haystack = `${item.title} ${item.snippet}`.toLowerCase();
    if (!terms.length) {
      return true;
    }
    return terms.some((term) => haystack.includes(term));
  });

  const byDomain = input.include_domains?.length
    ? filtered.filter((item) => input.include_domains?.some((domain) => item.url.includes(domain)))
    : filtered;

  const limit = input.max_results ?? 5;
  return {
    provider: 'exa',
    results: byDomain.slice(0, limit)
  };
}

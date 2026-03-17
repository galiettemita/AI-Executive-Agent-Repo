// Plan §6 step 3 — Real Tavily /search endpoint
// NOTE: auth is via api_key field in the JSON body, not a header

import type { TavilyInput, TavilyOutput } from './types.js';

interface TavilyResult {
  title: string;
  url: string;
  content: string;
  score: number;
}

interface TavilyApiResponse {
  results?: TavilyResult[];
}

export async function runClient(input: TavilyInput): Promise<TavilyOutput> {
  const key = process.env.TAVILY_API_KEY;
  if (!key) throw new Error('tavily: TAVILY_API_KEY not set');

  const response = await fetch('https://api.tavily.com/search', {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
    },
    body: JSON.stringify({
      api_key: key,
      query: input.query,
      max_results: input.max_results ?? 10,
      include_domains: input.include_domains ?? [],
      search_depth: 'basic',
    }),
    signal: AbortSignal.timeout(10000),
  });

  if (!response.ok) {
    const text = await response.text();
    throw new Error(`tavily: HTTP ${response.status} – ${text.slice(0, 300)}`);
  }

  const data = (await response.json()) as TavilyApiResponse;

  return {
    provider: 'tavily',
    results: (data.results ?? []).map((r) => ({
      title: r.title ?? '',
      url: r.url ?? '',
      content: r.content ?? '',
      score: r.score ?? 0,
    })),
  };
}

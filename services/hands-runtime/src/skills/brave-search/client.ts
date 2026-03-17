// Plan §6 step 1 — Real Brave Search API v1/web/search
// Replaces hardcoded fixture: brave-search previously returned 3 fake URLs

import type { BraveSearchInput, BraveSearchOutput } from './types.js';

interface BraveWebResult {
  title: string;
  url: string;
  description: string;
}

interface BraveApiResponse {
  web?: {
    results?: BraveWebResult[];
  };
}

export async function runClient(input: BraveSearchInput): Promise<BraveSearchOutput> {
  const key = process.env.BRAVE_SEARCH_API_KEY;
  if (!key) throw new Error('brave-search: BRAVE_SEARCH_API_KEY not set');

  const url = new URL('https://api.search.brave.com/res/v1/web/search');
  url.searchParams.set('q', input.query);
  url.searchParams.set('count', String(input.max_results ?? 10));

  const response = await fetch(url.toString(), {
    headers: {
      'X-Subscription-Token': key,
      'Accept': 'application/json',
      'Accept-Encoding': 'gzip',
    },
    signal: AbortSignal.timeout(10000),
  });

  if (!response.ok) {
    const text = await response.text();
    throw new Error(`brave-search: HTTP ${response.status} – ${text.slice(0, 300)}`);
  }

  const data = (await response.json()) as BraveApiResponse;

  return {
    provider: 'brave-search',
    results: (data.web?.results ?? []).map((r) => ({
      title: r.title ?? '',
      url: r.url ?? '',
      description: r.description ?? '',
    })),
  };
}

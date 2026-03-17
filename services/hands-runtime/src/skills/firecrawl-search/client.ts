// Plan §6 step 4 — Real Firecrawl /v1/search
// Response wraps items in data.data[] per plan mapping: data[].{title||url, url, markdown→content}

import type { FirecrawlInput, FirecrawlOutput } from './types.js';

interface FirecrawlItem {
  title?: string | null;
  url: string;
  markdown?: string | null;
}

interface FirecrawlApiResponse {
  data?: FirecrawlItem[];
}

export async function runClient(input: FirecrawlInput): Promise<FirecrawlOutput> {
  const key = process.env.FIRECRAWL_API_KEY ?? process.env.FAL_KEY;
  if (!key) throw new Error('firecrawl-search: FIRECRAWL_API_KEY not set');

  const response = await fetch('https://api.firecrawl.dev/v1/search', {
    method: 'POST',
    headers: {
      'Authorization': `Bearer ${key}`,
      'Content-Type': 'application/json',
    },
    body: JSON.stringify({
      query: input.query,
      limit: input.max_results ?? 10,
    }),
    signal: AbortSignal.timeout(10000),
  });

  if (!response.ok) {
    const text = await response.text();
    throw new Error(`firecrawl-search: HTTP ${response.status} – ${text.slice(0, 300)}`);
  }

  const data = (await response.json()) as FirecrawlApiResponse;

  return {
    provider: 'firecrawl',
    results: (data.data ?? []).map((item) => ({
      title: item.title ?? item.url,    // plan: title||url
      url: item.url,
      content: item.markdown ?? '',     // plan: markdown→content
    })),
  };
}

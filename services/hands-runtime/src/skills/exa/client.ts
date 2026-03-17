// Plan §6 step 2 — Real Exa /search endpoint

import type { ExaInput, ExaOutput } from './types.js';

interface ExaResult {
  title: string;
  url: string;
  text: string;
  score: number;
}

interface ExaApiResponse {
  results?: ExaResult[];
}

export async function runClient(input: ExaInput): Promise<ExaOutput> {
  const key = process.env.EXA_API_KEY;
  if (!key) throw new Error('exa: EXA_API_KEY not set');

  const response = await fetch('https://api.exa.ai/search', {
    method: 'POST',
    headers: {
      'x-api-key': key,
      'Content-Type': 'application/json',
    },
    body: JSON.stringify({
      query: input.query,
      numResults: input.max_results ?? 10,
      includeDomains: input.include_domains ?? [],
      useAutoprompt: true,
      contents: { text: { maxCharacters: 1000 } },
    }),
    signal: AbortSignal.timeout(10000),
  });

  if (!response.ok) {
    const text = await response.text();
    throw new Error(`exa: HTTP ${response.status} – ${text.slice(0, 300)}`);
  }

  const data = (await response.json()) as ExaApiResponse;

  return {
    provider: 'exa',
    results: (data.results ?? []).map((r) => ({
      title: r.title ?? '',
      url: r.url ?? '',
      snippet: r.text ?? '',   // plan: text→snippet
      score: r.score ?? 0,
    })),
  };
}

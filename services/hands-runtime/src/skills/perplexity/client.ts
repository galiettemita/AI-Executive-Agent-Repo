// Plan §6 step 5 — Real Perplexity /chat/completions
// citations[] → [{title:'Source N', url}] where N is 1-based index per plan

import type { PerplexityInput, PerplexityOutput } from './types.js';

interface PerplexityMessage {
  content: string;
}

interface PerplexityChoice {
  message: PerplexityMessage;
}

interface PerplexityApiResponse {
  choices?: PerplexityChoice[];
  citations?: string[];
}

export async function runClient(input: PerplexityInput): Promise<PerplexityOutput> {
  const key = process.env.PERPLEXITY_API_KEY;
  if (!key) throw new Error('perplexity: PERPLEXITY_API_KEY not set');

  const response = await fetch('https://api.perplexity.ai/chat/completions', {
    method: 'POST',
    headers: {
      'Authorization': `Bearer ${key}`,
      'Content-Type': 'application/json',
    },
    body: JSON.stringify({
      model: input.model || 'sonar',                // plan: input.model||'sonar'
      messages: [{ role: 'user', content: input.query }],
      return_citations: true,
    }),
    signal: AbortSignal.timeout(10000),
  });

  if (!response.ok) {
    const text = await response.text();
    throw new Error(`perplexity: HTTP ${response.status} – ${text.slice(0, 300)}`);
  }

  const data = (await response.json()) as PerplexityApiResponse;

  return {
    provider: 'perplexity',
    answer: data.choices?.[0]?.message?.content ?? '',
    citations: (data.citations ?? []).map((url, i) => ({
      title: `Source ${i + 1}`,    // plan: title:'Source N' (1-based)
      url,
    })),
  };
}

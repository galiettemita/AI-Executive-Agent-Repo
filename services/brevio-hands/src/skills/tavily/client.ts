import type { TavilyInput, TavilyOutput, TavilyResult } from './types.js';

function sanitizeToken(token: string): string {
  return token.toLowerCase().replace(/[^a-z0-9-]/g, '');
}

function buildUrl(token: string, domain?: string): string {
  const safeToken = sanitizeToken(token) || 'result';
  if (domain) {
    return `https://${domain.replace(/^https?:\/\//, '')}/${safeToken}`;
  }
  return `https://research.example.com/${safeToken}`;
}

function toResults(query: string, maxResults: number, domains: string[]): TavilyResult[] {
  const tokens = query
    .split(/\s+/)
    .map((token) => sanitizeToken(token))
    .filter((token) => token.length > 1);

  const baseTokens = tokens.length > 0 ? tokens : ['research'];

  const results: TavilyResult[] = [];
  for (let i = 0; i < maxResults; i += 1) {
    const token = baseTokens[i % baseTokens.length] ?? 'research';
    const domain = domains.length > 0 ? domains[i % domains.length] : undefined;
    const score = Math.max(0.1, Number((0.95 - i * 0.07).toFixed(2)));

    results.push({
      title: `${token} insight ${i + 1}`,
      url: buildUrl(token, domain),
      content: `Context summary for ${token} (${i + 1}/${maxResults}).`,
      score
    });
  }

  return results;
}

export async function runClient(input: TavilyInput): Promise<TavilyOutput> {
  const maxResults = input.max_results ?? 5;
  const domains = input.include_domains ?? [];

  return {
    provider: 'tavily',
    results: toResults(input.query, maxResults, domains)
  };
}

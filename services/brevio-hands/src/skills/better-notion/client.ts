import type { BetterNotionInput, BetterNotionOutput } from './types.js';

export async function runClient(input: BetterNotionInput): Promise<BetterNotionOutput> {
  const pages = [
    {
      page_id: input.page_id ?? 'notion-page-001',
      title: input.page_title ?? 'Notion Page',
      last_edited: '2026-03-04T18:20:00.000Z'
    }
  ];

  return {
    provider: 'better-notion',
    action: input.action,
    pages,
    summary: `Notion action ${input.action} completed with ${pages.length} page result(s).`
  };
}

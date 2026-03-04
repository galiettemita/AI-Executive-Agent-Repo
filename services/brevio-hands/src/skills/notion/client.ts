import { createHash } from 'node:crypto';

import type { NotionInput, NotionOutput, NotionPage } from './types.js';

const PAGES: NotionPage[] = [
  {
    page_id: 'page_exec_hub',
    title: 'Executive Hub',
    last_edited_time: '2026-02-21T11:05:00.000Z'
  },
  {
    page_id: 'page_q2_priorities',
    title: 'Q2 Priorities',
    last_edited_time: '2026-03-01T08:15:00.000Z'
  },
  {
    page_id: 'page_team_1on1',
    title: 'Team 1:1 Notes',
    last_edited_time: '2026-03-02T17:20:00.000Z'
  }
];

function buildPageId(seed: string): string {
  const hash = createHash('sha256').update(seed).digest('hex').slice(0, 14);
  return `page_${hash}`;
}

export async function runClient(input: NotionInput): Promise<NotionOutput> {
  if (input.action === 'search') {
    const query = input.query?.toLowerCase() ?? '';
    const pages = PAGES.filter((page) => page.title.toLowerCase().includes(query));
    return {
      provider: 'notion',
      action: 'search',
      pages
    };
  }

  if (input.action === 'create_page') {
    if (!input.title) {
      throw new Error('NOTION_TITLE_REQUIRED');
    }
    const pageId = buildPageId(`${input.title}|${input.content ?? ''}`);
    return {
      provider: 'notion',
      action: 'create_page',
      page_id: pageId,
      pages: [
        {
          page_id: pageId,
          title: input.title,
          last_edited_time: '2026-03-04T12:00:00.000Z'
        }
      ]
    };
  }

  if (!input.page_id || !input.content) {
    throw new Error('NOTION_APPEND_FIELDS_REQUIRED');
  }

  return {
    provider: 'notion',
    action: 'append_block',
    page_id: input.page_id,
    pages: [
      {
        page_id: input.page_id,
        title: PAGES.find((page) => page.page_id === input.page_id)?.title ?? 'Updated page',
        last_edited_time: '2026-03-04T12:05:00.000Z'
      }
    ]
  };
}

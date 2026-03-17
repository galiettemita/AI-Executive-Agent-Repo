// Plan §6 steps 8–9 — Real Notion /search, /pages
// Replaces: 3 fake page IDs

import type { NotionInput, NotionOutput } from './types.js';

interface NotionRichText {
  plain_text?: string;
  text?: { content?: string };
}

interface NotionPageResult {
  id: string;
  last_edited_time: string;
  properties: Record<string, { title?: NotionRichText[] } | undefined>;
}

interface NotionSearchResponse {
  results?: NotionPageResult[];
}

interface NotionCreateResponse {
  id: string;
}

// Plan §6 step 8: "title from properties.title[0].text.content"
function extractTitle(page: NotionPageResult): string {
  const titleProp = page.properties['title'] ?? page.properties['Name'] ?? page.properties['Title'];
  if (!titleProp?.title?.length) return '';
  const first = titleProp.title[0];
  return first?.text?.content ?? first?.plain_text ?? '';
}

export async function runClient(input: NotionInput): Promise<NotionOutput> {
  const token = process.env.NOTION_TOKEN;
  if (!token) throw new Error('notion: NOTION_TOKEN not set');

  const headers = {
    'Authorization': `Bearer ${token}`,
    'Notion-Version': '2022-06-28',
    'Content-Type': 'application/json',
  };

  if (input.action === 'search') {
    // Plan §6 step 8: POST /v1/search
    const response = await fetch('https://api.notion.com/v1/search', {
      method: 'POST',
      headers,
      body: JSON.stringify({
        query: input.query ?? '',
        filter: { property: 'object', value: 'page' },
        page_size: 10,
      }),
      signal: AbortSignal.timeout(10000),
    });

    if (!response.ok) {
      const text = await response.text();
      throw new Error(`notion: HTTP ${response.status} – ${text.slice(0, 300)}`);
    }

    const data = (await response.json()) as NotionSearchResponse;

    return {
      provider: 'notion',
      action: 'search',
      pages: (data.results ?? []).map((page) => ({
        page_id: page.id,
        title: extractTitle(page),
        last_edited_time: page.last_edited_time,
      })),
    };
  }

  if (input.action === 'create_page') {
    if (!input.title) throw new Error('notion: title is required for create_page');

    // Plan §6 step 9: POST /v1/pages
    const response = await fetch('https://api.notion.com/v1/pages', {
      method: 'POST',
      headers,
      body: JSON.stringify({
        parent: { type: 'workspace', workspace: true },
        properties: {
          title: [{ text: { content: input.title } }],
        },
        children: input.content
          ? [
              {
                object: 'block',
                type: 'paragraph',
                paragraph: {
                  rich_text: [{ type: 'text', text: { content: input.content } }],
                },
              },
            ]
          : [],
      }),
      signal: AbortSignal.timeout(10000),
    });

    if (!response.ok) {
      const text = await response.text();
      throw new Error(`notion: HTTP ${response.status} – ${text.slice(0, 300)}`);
    }

    const data = (await response.json()) as NotionCreateResponse;
    return {
      provider: 'notion',
      action: 'create_page',
      page_id: data.id,
    };
  }

  if (input.action === 'append_block') {
    if (!input.page_id || !input.content) {
      throw new Error('notion: page_id and content are required for append_block');
    }

    const response = await fetch(
      `https://api.notion.com/v1/blocks/${input.page_id}/children`,
      {
        method: 'PATCH',
        headers,
        body: JSON.stringify({
          children: [
            {
              object: 'block',
              type: 'paragraph',
              paragraph: {
                rich_text: [{ type: 'text', text: { content: input.content } }],
              },
            },
          ],
        }),
        signal: AbortSignal.timeout(10000),
      }
    );

    if (!response.ok) {
      const text = await response.text();
      throw new Error(`notion: HTTP ${response.status} – ${text.slice(0, 300)}`);
    }

    return {
      provider: 'notion',
      action: 'append_block',
      page_id: input.page_id,
    };
  }

  throw new Error(`notion: unknown action ${input.action}`);
}

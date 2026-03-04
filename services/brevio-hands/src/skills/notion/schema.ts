import { z } from 'zod';

export const InputSchema = z
  .object({
    action: z.enum(['search', 'create_page', 'append_block']),
    query: z.string().min(2).max(300).optional(),
    page_id: z.string().min(3).max(120).optional(),
    title: z.string().min(1).max(300).optional(),
    content: z.string().min(1).max(5000).optional()
  })
  .strict();

const NotionPageSchema = z.object({
  page_id: z.string(),
  title: z.string(),
  last_edited_time: z.string().datetime()
});

export const OutputSchema = z
  .object({
    provider: z.literal('notion'),
    action: z.enum(['search', 'create_page', 'append_block']),
    page_id: z.string().optional(),
    pages: z.array(NotionPageSchema).optional()
  })
  .strict();

import { z } from 'zod';

const ActionSchema = z.enum(['create_page', 'query_database', 'update_page']);

export const InputSchema = z
  .object({
    action: ActionSchema,
    page_title: z.string().min(2).max(300).optional(),
    page_id: z.string().min(2).max(120).optional(),
    database_id: z.string().min(2).max(120).optional(),
    content: z.string().min(2).max(20000).optional()
  })
  .strict()
  .superRefine((value, ctx) => {
    if (value.action === 'create_page' && !value.page_title) {
      ctx.addIssue({ code: z.ZodIssueCode.custom, message: 'BETTER_NOTION_CREATE_TITLE_REQUIRED' });
    }
    if (value.action === 'update_page' && !value.page_id) {
      ctx.addIssue({ code: z.ZodIssueCode.custom, message: 'BETTER_NOTION_PAGE_ID_REQUIRED' });
    }
    if (value.action === 'query_database' && !value.database_id) {
      ctx.addIssue({ code: z.ZodIssueCode.custom, message: 'BETTER_NOTION_DATABASE_REQUIRED' });
    }
  });

export const OutputSchema = z
  .object({
    provider: z.literal('better-notion'),
    action: ActionSchema,
    pages: z.array(
      z
        .object({
          page_id: z.string().min(2).max(120),
          title: z.string().min(2).max(300),
          last_edited: z.string().datetime()
        })
        .strict()
    ),
    summary: z.string().min(10).max(4096)
  })
  .strict();

import { z } from 'zod';

const ActionSchema = z.enum(['create_doc', 'append_doc', 'search_docs']);

export const InputSchema = z
  .object({
    action: ActionSchema,
    doc_title: z.string().min(2).max(240).optional(),
    doc_id: z.string().min(2).max(120).optional(),
    content: z.string().min(2).max(20000).optional(),
    query: z.string().min(2).max(500).optional()
  })
  .strict()
  .superRefine((value, ctx) => {
    if (value.action === 'create_doc' && !value.doc_title) {
      ctx.addIssue({ code: z.ZodIssueCode.custom, message: 'CRAFT_DOC_TITLE_REQUIRED' });
    }
    if (value.action === 'append_doc' && !value.doc_id) {
      ctx.addIssue({ code: z.ZodIssueCode.custom, message: 'CRAFT_DOC_ID_REQUIRED' });
    }
    if (value.action === 'search_docs' && !value.query) {
      ctx.addIssue({ code: z.ZodIssueCode.custom, message: 'CRAFT_QUERY_REQUIRED' });
    }
  });

export const OutputSchema = z
  .object({
    provider: z.literal('craft'),
    action: ActionSchema,
    docs: z.array(
      z
        .object({
          doc_id: z.string().min(2).max(120),
          title: z.string().min(2).max(240),
          updated_at: z.string().datetime()
        })
        .strict()
    ),
    summary: z.string().min(10).max(4096)
  })
  .strict();

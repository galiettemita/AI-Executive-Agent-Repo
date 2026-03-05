import { z } from 'zod';

const ActionSchema = z.enum(['search_all', 'search_sender', 'search_subject']);

const ResultSchema = z.object({
  message_id: z.string().min(2).max(120),
  mailbox: z.string().min(1).max(120),
  sender: z.string().email(),
  subject: z.string().min(1).max(240),
  snippet: z.string().min(1).max(500),
  received_at: z.string().datetime()
});

export const InputSchema = z
  .object({
    action: ActionSchema,
    query: z.string().min(2).max(200).optional(),
    mailbox: z.string().min(1).max(120).optional(),
    limit: z.number().int().min(1).max(100).optional()
  })
  .strict()
  .superRefine((value, ctx) => {
    if (!value.query?.trim()) {
      ctx.addIssue({ code: z.ZodIssueCode.custom, message: 'APPLE_MAIL_SEARCH_QUERY_REQUIRED' });
    }
  });

export const OutputSchema = z
  .object({
    provider: z.literal('apple-mail-search'),
    action: ActionSchema,
    query: z.string().min(2).max(200),
    results: z.array(ResultSchema).max(200),
    latency_profile_ms: z.literal(50),
    summary: z.string().min(10).max(4096)
  })
  .strict();

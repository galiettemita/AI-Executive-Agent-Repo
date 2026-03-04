import { z } from 'zod';

const ActionSchema = z.enum(['draft_post', 'generate_thread']);
const PlatformSchema = z.enum(['x', 'linkedin', 'bluesky']);

export const InputSchema = z
  .object({
    action: ActionSchema,
    journal_entry: z.string().min(20).max(10000).optional(),
    platform: PlatformSchema.optional()
  })
  .strict()
  .superRefine((value, ctx) => {
    if (!value.journal_entry) {
      ctx.addIssue({ code: z.ZodIssueCode.custom, message: 'JOURNAL_TO_POST_ENTRY_REQUIRED' });
    }
  });

export const OutputSchema = z
  .object({
    provider: z.literal('journal-to-post'),
    action: ActionSchema,
    platform: PlatformSchema,
    post_text: z.string().min(20).max(3000),
    thread_parts: z.array(z.string().min(5).max(500)).max(20),
    summary: z.string().min(10).max(4096)
  })
  .strict();

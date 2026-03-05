import { z } from 'zod';

const EmailSchema = z.object({
  id: z.string(),
  from: z.string().email(),
  to: z.array(z.string().email()).min(1),
  subject: z.string(),
  snippet: z.string(),
  received_at: z.string().datetime()
});

const ActionSchema = z.enum(['list_inbox', 'search', 'send', 'reply']);

export const InputSchema = z
  .object({
    action: ActionSchema,
    query: z.string().min(2).max(400).optional(),
    to: z.array(z.string().email()).min(1).optional(),
    subject: z.string().min(1).max(240).optional(),
    body: z.string().min(1).max(4000).optional(),
    reply_to_id: z.string().min(3).max(120).optional(),
    confirmed: z.boolean().optional()
  })
  .strict()
  .superRefine((value, ctx) => {
    if (value.action === 'search' && !value.query) {
      ctx.addIssue({
        code: z.ZodIssueCode.custom,
        message: 'APPLE_MAIL_SEARCH_QUERY_REQUIRED'
      });
    }

    if (value.action === 'send') {
      if (!value.to || !value.subject || !value.body) {
        ctx.addIssue({
          code: z.ZodIssueCode.custom,
          message: 'APPLE_MAIL_SEND_FIELDS_REQUIRED'
        });
      }
    }

    if (value.action === 'reply') {
      if (!value.reply_to_id || !value.body) {
        ctx.addIssue({
          code: z.ZodIssueCode.custom,
          message: 'APPLE_MAIL_REPLY_FIELDS_REQUIRED'
        });
      }
    }
  });

export const OutputSchema = z
  .object({
    provider: z.literal('apple-mail-local'),
    action: ActionSchema,
    emails: z.array(EmailSchema).optional(),
    sent: z.boolean().optional(),
    message_id: z.string().optional()
  })
  .strict();

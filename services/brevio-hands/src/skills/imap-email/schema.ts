import { z } from 'zod';

const MessageSchema = z.object({
  id: z.string(),
  from: z.string().email(),
  subject: z.string(),
  snippet: z.string(),
  received_at: z.string().datetime()
});

const ActionSchema = z.enum(['list', 'search', 'send']);

export const InputSchema = z
  .object({
    action: ActionSchema,
    mailbox: z.string().min(2).max(120).optional(),
    query: z.string().min(2).max(400).optional(),
    to: z.array(z.string().email()).min(1).optional(),
    subject: z.string().min(1).max(240).optional(),
    body: z.string().min(1).max(4000).optional(),
    confirmed: z.boolean().optional()
  })
  .strict()
  .superRefine((value, ctx) => {
    if (value.action === 'search' && !value.query) {
      ctx.addIssue({
        code: z.ZodIssueCode.custom,
        message: 'IMAP_EMAIL_SEARCH_QUERY_REQUIRED'
      });
    }
    if (value.action === 'send' && (!value.to || !value.subject || !value.body)) {
      ctx.addIssue({
        code: z.ZodIssueCode.custom,
        message: 'IMAP_EMAIL_SEND_FIELDS_REQUIRED'
      });
    }
  });

export const OutputSchema = z
  .object({
    provider: z.literal('imap-email'),
    action: ActionSchema,
    mailbox: z.string(),
    messages: z.array(MessageSchema).optional(),
    sent: z.boolean().optional()
  })
  .strict();

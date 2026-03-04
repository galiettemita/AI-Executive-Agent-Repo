import { z } from 'zod';

export const InputSchema = z
  .object({
    action: z.enum(['inbox_list', 'send', 'calendar_list']),
    to: z.array(z.string().email()).min(1).max(20).optional(),
    subject: z.string().min(1).max(255).optional(),
    body: z.string().min(1).max(50000).optional(),
    confirmed: z.boolean().optional()
  })
  .strict();

const MailSchema = z.object({
  message_id: z.string(),
  from: z.string().email(),
  subject: z.string()
});

const EventSchema = z.object({
  event_id: z.string(),
  subject: z.string(),
  start_time: z.string().datetime()
});

export const OutputSchema = z
  .object({
    provider: z.literal('outlook'),
    action: z.enum(['inbox_list', 'send', 'calendar_list']),
    confirmation_required: z.boolean().optional(),
    message_id: z.string().optional(),
    mails: z.array(MailSchema).optional(),
    events: z.array(EventSchema).optional()
  })
  .strict();

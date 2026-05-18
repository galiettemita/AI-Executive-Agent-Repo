import { z } from 'zod';

export const InputSchema = z
  .object({
    action: z.enum(['gmail_list', 'gmail_send', 'calendar_list', 'drive_search']),
    query: z.string().min(1).max(300).optional(),
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
  title: z.string(),
  start_time: z.string().datetime()
});

const FileSchema = z.object({
  file_id: z.string(),
  name: z.string(),
  mime_type: z.string()
});

export const OutputSchema = z
  .object({
    provider: z.literal('google-workspace'),
    action: z.enum(['gmail_list', 'gmail_send', 'calendar_list', 'drive_search']),
    confirmation_required: z.boolean().optional(),
    message_id: z.string().optional(),
    mails: z.array(MailSchema).optional(),
    events: z.array(EventSchema).optional(),
    files: z.array(FileSchema).optional()
  })
  .strict();

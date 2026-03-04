import { z } from 'zod';

export const InputSchema = z.object({
  to: z.array(z.string().email()).min(1).max(25),
  subject: z.string().min(1).max(255),
  body: z.string().min(1).max(50000),
  html: z.string().max(200000).optional(),
  confirmed: z.boolean().optional()
});

export const OutputSchema = z.object({
  message_id: z.string(),
  sent: z.boolean(),
  confirmation_required: z.boolean(),
  recipients: z.array(z.string().email())
});

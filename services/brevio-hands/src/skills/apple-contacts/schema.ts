import { z } from 'zod';

const ContactSchema = z.object({
  name: z.string(),
  phone: z.string().optional(),
  email: z.string().email().optional()
});

export const InputSchema = z
  .object({
    query: z.string().min(2).max(200)
  })
  .strict();

export const OutputSchema = z
  .object({
    provider: z.literal('apple-contacts-local'),
    contacts: z.array(ContactSchema)
  })
  .strict();

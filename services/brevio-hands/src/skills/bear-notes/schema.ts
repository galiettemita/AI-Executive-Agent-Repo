import { z } from 'zod';

const NoteSchema = z.object({
  note_id: z.string(),
  title: z.string(),
  content_preview: z.string(),
  updated_at: z.string().datetime()
});

export const InputSchema = z
  .object({
    action: z.enum(['list', 'create', 'search', 'update']),
    note_id: z.string().min(2).max(120).optional(),
    title: z.string().min(1).max(300).optional(),
    content: z.string().min(1).max(10000).optional(),
    query: z.string().min(1).max(300).optional()
  })
  .strict();

export const OutputSchema = z
  .object({
    provider: z.literal('bear-notes'),
    action: z.enum(['list', 'create', 'search', 'update']),
    note_id: z.string().optional(),
    notes: z.array(NoteSchema).optional()
  })
  .strict();

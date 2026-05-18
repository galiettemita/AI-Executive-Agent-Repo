import { z } from 'zod';

const ActionSchema = z.enum(['create_note', 'search_notes', 'list_recent']);

const NoteSchema = z.object({
  note_id: z.string().min(2).max(80),
  title: z.string().min(1).max(200),
  folder: z.string().min(1).max(120),
  updated_at: z.string().datetime(),
  preview: z.string().min(1).max(500)
});

export const InputSchema = z
  .object({
    action: ActionSchema,
    title: z.string().min(1).max(200).optional(),
    body: z.string().min(1).max(5000).optional(),
    folder: z.string().min(1).max(120).optional(),
    query: z.string().min(2).max(200).optional()
  })
  .strict()
  .superRefine((value, ctx) => {
    if (value.action === 'create_note' && (!value.title || !value.body)) {
      ctx.addIssue({ code: z.ZodIssueCode.custom, message: 'APPLE_NOTES_CREATE_FIELDS_REQUIRED' });
    }

    if (value.action === 'search_notes' && !value.query) {
      ctx.addIssue({ code: z.ZodIssueCode.custom, message: 'APPLE_NOTES_QUERY_REQUIRED' });
    }
  });

export const OutputSchema = z
  .object({
    provider: z.literal('apple-notes'),
    action: ActionSchema,
    canonical_skill_id: z.literal('apple-notes-skill'),
    deprecated_alias: z.literal(true),
    notes: z.array(NoteSchema).max(200),
    summary: z.string().min(10).max(4096)
  })
  .strict();

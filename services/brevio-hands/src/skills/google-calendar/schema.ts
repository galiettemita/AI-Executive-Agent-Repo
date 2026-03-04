import { z } from 'zod';

export const InputSchema = z.object({
  action: z.enum(['list', 'create', 'update', 'delete']),
  calendar_id: z.string().min(1).max(200).optional(),
  confirmed: z.boolean().optional(),
  event: z
    .object({
      event_id: z.string().min(2).max(120).optional(),
      title: z.string().min(1).max(200).optional(),
      start_time: z.string().datetime().optional(),
      end_time: z.string().datetime().optional(),
      description: z.string().max(2000).optional(),
      location: z.string().max(500).optional()
    })
    .optional()
});

export const OutputSchema = z.object({
  action: z.enum(['list', 'create', 'update', 'delete']),
  calendar_id: z.string(),
  events: z
    .array(
      z.object({
        event_id: z.string(),
        title: z.string(),
        start_time: z.string().datetime(),
        end_time: z.string().datetime(),
        status: z.enum(['scheduled', 'updated', 'deleted'])
      })
    )
    .optional(),
  event_id: z.string().optional(),
  confirmation_required: z.boolean()
});

import { z } from 'zod';

const TodoItemSchema = z.object({
  item_id: z.string(),
  content: z.string(),
  due: z.string().optional(),
  completed: z.boolean()
});

export const InputSchema = z
  .object({
    action: z.enum(['list', 'add', 'complete', 'delete']),
    item_id: z.string().min(3).max(80).optional(),
    content: z.string().min(1).max(500).optional(),
    due: z.string().max(120).optional()
  })
  .strict();

export const OutputSchema = z
  .object({
    provider: z.literal('todo'),
    action: z.enum(['list', 'add', 'complete', 'delete']),
    item_id: z.string().optional(),
    items: z.array(TodoItemSchema).optional()
  })
  .strict();

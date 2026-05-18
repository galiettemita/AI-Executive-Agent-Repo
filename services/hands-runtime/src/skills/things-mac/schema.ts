import { z } from 'zod';

const ActionSchema = z.enum(['create_todo', 'list_today', 'complete_todo', 'move_to_project']);

export const InputSchema = z
  .object({
    action: ActionSchema,
    title: z.string().min(2).max(240).optional(),
    todo_id: z.string().min(3).max(120).optional(),
    project: z.string().min(1).max(120).optional(),
    due_date: z.string().datetime().optional()
  })
  .strict()
  .superRefine((value, ctx) => {
    if (value.action === 'create_todo' && !value.title) {
      ctx.addIssue({ code: z.ZodIssueCode.custom, message: 'THINGS_MAC_TITLE_REQUIRED' });
    }
    if ((value.action === 'complete_todo' || value.action === 'move_to_project') && !value.todo_id) {
      ctx.addIssue({ code: z.ZodIssueCode.custom, message: 'THINGS_MAC_TODO_REQUIRED' });
    }
    if (value.action === 'move_to_project' && !value.project) {
      ctx.addIssue({ code: z.ZodIssueCode.custom, message: 'THINGS_MAC_PROJECT_REQUIRED' });
    }
  });

export const OutputSchema = z
  .object({
    provider: z.literal('things-mac'),
    action: ActionSchema,
    todos: z.array(
      z
        .object({
          todo_id: z.string().min(3).max(120),
          title: z.string().min(1).max(240),
          project: z.string().min(1).max(120),
          status: z.enum(['open', 'completed']),
          due_date: z.string().datetime().optional()
        })
        .strict()
    ),
    inbox_count: z.number().int().min(0).max(10000),
    summary: z.string().min(10).max(4096)
  })
  .strict();

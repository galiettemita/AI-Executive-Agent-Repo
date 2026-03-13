import { z } from 'zod';

const TodoistTaskInputSchema = z.object({
  task_id: z.string().min(3).max(80).optional(),
  content: z.string().min(1).max(500).optional(),
  due_string: z.string().max(120).optional(),
  priority: z.union([z.literal(1), z.literal(2), z.literal(3), z.literal(4)]).optional()
});

export const InputSchema = z
  .object({
    action: z.enum(['list', 'create', 'complete', 'delete']),
    project_id: z.string().min(1).max(120).optional(),
    task: TodoistTaskInputSchema.optional()
  })
  .strict();

const TodoistTaskSchema = z.object({
  task_id: z.string(),
  content: z.string(),
  project_id: z.string(),
  due_string: z.string().optional(),
  priority: z.number().int().min(1).max(4),
  completed: z.boolean()
});

export const OutputSchema = z
  .object({
    provider: z.literal('todoist_deterministic'),
    action: z.enum(['list', 'create', 'complete', 'delete']),
    task_id: z.string().optional(),
    tasks: z.array(TodoistTaskSchema).optional()
  })
  .strict();

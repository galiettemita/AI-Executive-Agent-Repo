import { z } from 'zod';

const TaskSchema = z.object({
  task_id: z.string(),
  name: z.string(),
  project_id: z.string(),
  status: z.enum(['todo', 'in_progress', 'done'])
});

export const InputSchema = z
  .object({
    action: z.enum(['task_list', 'task_create', 'task_update']),
    project_id: z.string().min(2).max(120).optional(),
    task_id: z.string().min(2).max(120).optional(),
    name: z.string().min(1).max(300).optional(),
    notes: z.string().max(5000).optional(),
    status: z.enum(['todo', 'in_progress', 'done']).optional()
  })
  .strict();

export const OutputSchema = z
  .object({
    provider: z.literal('asana'),
    action: z.enum(['task_list', 'task_create', 'task_update']),
    task_id: z.string().optional(),
    tasks: z.array(TaskSchema).optional()
  })
  .strict();

import { z } from 'zod';

const TaskSchema = z.object({
  task_id: z.string(),
  title: z.string(),
  status: z.string()
});

export const InputSchema = z
  .object({
    action: z.enum(['task_list', 'task_create', 'doc_create', 'time_start', 'time_stop']),
    list_id: z.string().min(2).max(120).optional(),
    task_id: z.string().min(2).max(120).optional(),
    title: z.string().min(1).max(300).optional(),
    content: z.string().max(5000).optional()
  })
  .strict();

export const OutputSchema = z
  .object({
    provider: z.literal('clickup-mcp'),
    action: z.enum(['task_list', 'task_create', 'doc_create', 'time_start', 'time_stop']),
    task_id: z.string().optional(),
    doc_id: z.string().optional(),
    timer_started: z.boolean().optional(),
    tasks: z.array(TaskSchema).optional()
  })
  .strict();

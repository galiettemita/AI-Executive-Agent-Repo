import type { TickTickInput, TickTickOutput, TickTickTask } from './types.js';

const BASE_TASKS: TickTickTask[] = [
  {
    task_id: 'tt-1',
    content: 'Review board deck',
    project_id: 'work',
    status: 'open',
    due_date: '2026-03-05T18:00:00.000Z'
  },
  {
    task_id: 'tt-2',
    content: 'Book dentist appointment',
    project_id: 'personal',
    status: 'open'
  }
];

function buildTask(input: TickTickInput): TickTickTask {
  return {
    task_id: input.task_id ?? `tt-${(input.task_content ?? 'task').toLowerCase().replace(/[^a-z0-9]+/g, '-').slice(0, 18)}`,
    content: input.task_content ?? 'Updated TickTick task',
    project_id: input.project_id ?? 'inbox',
    status: input.action === 'complete_task' ? 'completed' : 'open',
    due_date: input.due_date
  };
}

export async function runClient(input: TickTickInput): Promise<TickTickOutput> {
  if (input.action === 'list_tasks') {
    return {
      provider: 'ticktick',
      action: input.action,
      tasks: BASE_TASKS,
      total_tasks: BASE_TASKS.length,
      summary: `Retrieved ${BASE_TASKS.length} TickTick tasks.`
    };
  }

  const task = buildTask(input);
  const tasks = input.action === 'delete_task' ? [] : [task];

  return {
    provider: 'ticktick',
    action: input.action,
    tasks,
    total_tasks: tasks.length,
    summary: `TickTick action ${input.action} applied to task ${task.task_id}.`
  };
}

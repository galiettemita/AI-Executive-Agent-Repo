import type { OmnifocusInput, OmnifocusOutput, OmnifocusTask } from './types.js';

const FLAGGED_TASKS: OmnifocusTask[] = [
  {
    task_id: 'of-1',
    title: 'Draft strategy memo',
    project: 'Exec Planning',
    status: 'available'
  },
  {
    task_id: 'of-2',
    title: 'Approve legal redlines',
    project: 'Operations',
    status: 'available'
  }
];

function buildTask(input: OmnifocusInput): OmnifocusTask {
  return {
    task_id: input.task_id ?? `of-${(input.title ?? 'task').toLowerCase().replace(/[^a-z0-9]+/g, '-').slice(0, 18)}`,
    title: input.title ?? 'OmniFocus task',
    project: input.project ?? 'Inbox',
    status: input.action === 'complete_task' ? 'completed' : input.action === 'defer_task' ? 'deferred' : 'available',
    defer_until: input.defer_until
  };
}

export async function runClient(input: OmnifocusInput): Promise<OmnifocusOutput> {
  if (input.action === 'list_flagged') {
    return {
      provider: 'omnifocus',
      action: input.action,
      tasks: FLAGGED_TASKS,
      flagged_count: FLAGGED_TASKS.length,
      summary: `Loaded ${FLAGGED_TASKS.length} flagged OmniFocus tasks.`
    };
  }

  const task = buildTask(input);
  return {
    provider: 'omnifocus',
    action: input.action,
    tasks: [task],
    flagged_count: input.action === 'add_task' ? FLAGGED_TASKS.length + 1 : FLAGGED_TASKS.length,
    summary: `OmniFocus action ${input.action} applied to ${task.task_id}.`
  };
}

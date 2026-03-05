import { createHash } from 'node:crypto';

import type { TodoistInput, TodoistOutput, TodoistTask } from './types.js';

const BASE_TASKS: TodoistTask[] = [
  {
    task_id: 'task_exec_review',
    content: 'Review executive briefing draft',
    project_id: 'inbox',
    due_string: 'today 5pm',
    priority: 4,
    completed: false
  },
  {
    task_id: 'task_budget_sync',
    content: 'Sync Q2 budget notes',
    project_id: 'finance',
    due_string: 'tomorrow 10am',
    priority: 3,
    completed: false
  }
];

function buildTaskId(seed: string): string {
  const hash = createHash('sha256').update(seed).digest('hex').slice(0, 16);
  return `task_${hash}`;
}

export async function runClient(input: TodoistInput): Promise<TodoistOutput> {
  const tasks = BASE_TASKS.map((task) => ({ ...task }));
  const projectId = input.project_id ?? 'inbox';

  if (input.action === 'list') {
    const filtered = tasks.filter((task) => task.project_id === projectId || projectId === 'inbox');
    return {
      provider: 'todoist_mock',
      action: 'list',
      tasks: filtered
    };
  }

  if (input.action === 'create') {
    const content = input.task?.content?.trim();
    if (!content) {
      throw new Error('TODOIST_CONTENT_REQUIRED');
    }

    const taskId = buildTaskId(`${projectId}|${content}|${input.task?.due_string ?? ''}`);
    return {
      provider: 'todoist_mock',
      action: 'create',
      task_id: taskId,
      tasks: [
        {
          task_id: taskId,
          content,
          project_id: projectId,
          due_string: input.task?.due_string,
          priority: input.task?.priority ?? 1,
          completed: false
        }
      ]
    };
  }

  const taskId = input.task?.task_id?.trim();
  if (!taskId) {
    throw new Error('TODOIST_TASK_ID_REQUIRED');
  }

  if (input.action === 'complete') {
    return {
      provider: 'todoist_mock',
      action: 'complete',
      task_id: taskId,
      tasks: [
        {
          task_id: taskId,
          content: 'Completed task',
          project_id: projectId,
          priority: 1,
          completed: true
        }
      ]
    };
  }

  return {
    provider: 'todoist_mock',
    action: 'delete',
    task_id: taskId
  };
}

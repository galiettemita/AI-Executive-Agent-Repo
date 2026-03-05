import { createHash } from 'node:crypto';

import type { AsanaInput, AsanaOutput, AsanaTask } from './types.js';

const TASKS: AsanaTask[] = [
  {
    task_id: 'asn_001',
    name: 'Draft quarterly strategy memo',
    project_id: 'proj_exec',
    status: 'in_progress'
  },
  {
    task_id: 'asn_002',
    name: 'Review roadmap dependencies',
    project_id: 'proj_exec',
    status: 'todo'
  }
];

function taskId(seed: string): string {
  return `asn_${createHash('sha256').update(seed).digest('hex').slice(0, 8)}`;
}

export async function runClient(input: AsanaInput): Promise<AsanaOutput> {
  if (input.action === 'task_list') {
    return {
      provider: 'asana',
      action: 'task_list',
      tasks: TASKS.filter((task) => !input.project_id || task.project_id === input.project_id)
    };
  }

  if (input.action === 'task_create') {
    if (!input.project_id || !input.name) {
      throw new Error('ASANA_CREATE_FIELDS_REQUIRED');
    }

    return {
      provider: 'asana',
      action: 'task_create',
      task_id: taskId(`${input.project_id}|${input.name}`),
      tasks: [
        {
          task_id: taskId(`${input.project_id}|${input.name}`),
          name: input.name,
          project_id: input.project_id,
          status: input.status ?? 'todo'
        }
      ]
    };
  }

  if (!input.task_id) {
    throw new Error('ASANA_TASK_ID_REQUIRED');
  }

  return {
    provider: 'asana',
    action: 'task_update',
    task_id: input.task_id,
    tasks: [
      {
        task_id: input.task_id,
        name: input.name ?? 'Updated task',
        project_id: input.project_id ?? 'proj_exec',
        status: input.status ?? 'in_progress'
      }
    ]
  };
}

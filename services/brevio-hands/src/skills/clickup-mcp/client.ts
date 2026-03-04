import { createHash } from 'node:crypto';

import type { ClickupInput, ClickupOutput, ClickupTask } from './types.js';

const TASKS: ClickupTask[] = [
  {
    task_id: 'clk_001',
    title: 'Refine onboarding workflow',
    status: 'open'
  },
  {
    task_id: 'clk_002',
    title: 'Audit connector coverage',
    status: 'in_progress'
  }
];

function id(prefix: string, seed: string): string {
  return `${prefix}_${createHash('sha256').update(seed).digest('hex').slice(0, 8)}`;
}

export async function runClient(input: ClickupInput): Promise<ClickupOutput> {
  if (input.action === 'task_list') {
    return {
      provider: 'clickup-mcp',
      action: 'task_list',
      tasks: TASKS
    };
  }

  if (input.action === 'task_create') {
    if (!input.title) {
      throw new Error('CLICKUP_TITLE_REQUIRED');
    }
    return {
      provider: 'clickup-mcp',
      action: 'task_create',
      task_id: id('clk', input.title),
      tasks: [
        {
          task_id: id('clk', input.title),
          title: input.title,
          status: 'open'
        }
      ]
    };
  }

  if (input.action === 'doc_create') {
    if (!input.title) {
      throw new Error('CLICKUP_DOC_TITLE_REQUIRED');
    }
    return {
      provider: 'clickup-mcp',
      action: 'doc_create',
      doc_id: id('doc', input.title)
    };
  }

  if (!input.task_id) {
    throw new Error('CLICKUP_TASK_ID_REQUIRED');
  }

  return {
    provider: 'clickup-mcp',
    action: input.action,
    task_id: input.task_id,
    timer_started: input.action === 'time_start'
  };
}

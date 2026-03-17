// Plan §6 steps 10–11 — Real Asana /tasks endpoints
// Replaces: fictional issues

import type { AsanaInput, AsanaOutput, AsanaTask } from './types.js';

const ASANA_BASE = 'https://app.asana.com/api/1.0';

interface AsanaTaskData {
  gid: string;
  name: string;
  completed: boolean;
}

interface AsanaListResponse {
  data?: AsanaTaskData[];
}

interface AsanaCreateResponse {
  data?: { gid: string };
}

// Plan: completed→status: true→'done', false→'todo'
function mapStatus(completed: boolean): AsanaTask['status'] {
  return completed ? 'done' : 'todo';
}

export async function runClient(input: AsanaInput): Promise<AsanaOutput> {
  const token = process.env.ASANA_ACCESS_TOKEN;
  if (!token) throw new Error('asana: ASANA_ACCESS_TOKEN not set');

  const headers = {
    'Authorization': `Bearer ${token}`,
    'Accept': 'application/json',
    'Content-Type': 'application/json',
  };

  if (input.action === 'task_list') {
    // Plan §6 step 10: GET /tasks?project=<id>&limit=20
    const projectId = input.project_id ?? '';
    const url = new URL(`${ASANA_BASE}/tasks`);
    url.searchParams.set('project', projectId);
    url.searchParams.set('limit', '20');
    url.searchParams.set('opt_fields', 'gid,name,completed');

    const response = await fetch(url.toString(), {
      headers,
      signal: AbortSignal.timeout(10000),
    });

    if (!response.ok) {
      const text = await response.text();
      throw new Error(`asana: HTTP ${response.status} – ${text.slice(0, 300)}`);
    }

    const data = (await response.json()) as AsanaListResponse;

    return {
      provider: 'asana',
      action: 'task_list',
      tasks: (data.data ?? []).map((t) => ({
        task_id: t.gid,                       // plan: gid→task_id
        name: t.name,
        project_id: projectId,                // plan: project_id (from input)
        status: mapStatus(t.completed),       // plan: completed→status
      })),
    };
  }

  if (input.action === 'task_create') {
    if (!input.project_id || !input.name) {
      throw new Error('asana: project_id and name are required for task_create');
    }

    // Plan §6 step 11: POST /tasks with name, notes, projects:[project_id]
    const response = await fetch(`${ASANA_BASE}/tasks`, {
      method: 'POST',
      headers,
      body: JSON.stringify({
        data: {
          name: input.name,
          notes: input.notes ?? '',
          projects: [input.project_id],
        },
      }),
      signal: AbortSignal.timeout(10000),
    });

    if (!response.ok) {
      const text = await response.text();
      throw new Error(`asana: HTTP ${response.status} – ${text.slice(0, 300)}`);
    }

    const data = (await response.json()) as AsanaCreateResponse;
    const taskId = data.data?.gid ?? '';

    return {
      provider: 'asana',
      action: 'task_create',
      task_id: taskId,
      tasks: [
        {
          task_id: taskId,
          name: input.name,
          project_id: input.project_id,
          status: 'todo',
        },
      ],
    };
  }

  throw new Error(`asana: unknown action ${input.action}`);
}

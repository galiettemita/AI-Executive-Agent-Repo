// Plan §6 steps 17–19 — Real Todoist /rest/v2/tasks
// HTTP 204 checks for complete and delete (plan §6 step 19 explicit)

import type { TodoistInput, TodoistOutput } from './types.js';

const TODOIST_BASE = 'https://api.todoist.com/rest/v2';

interface TodoistApiTask {
  id: string;
  content: string;
  project_id: string;
  due?: { string?: string } | null;
  priority: number;
  is_completed: boolean;
}

export async function runClient(input: TodoistInput): Promise<TodoistOutput> {
  const token = process.env.TODOIST_API_TOKEN;
  if (!token) throw new Error('todoist: TODOIST_API_TOKEN not set');

  const headers = {
    'Authorization': `Bearer ${token}`,
    'Content-Type': 'application/json',
  };

  const projectId = input.project_id ?? 'inbox';

  if (input.action === 'list') {
    // Plan §6 step 17: GET /tasks?project_id=<id>
    const url = new URL(`${TODOIST_BASE}/tasks`);
    url.searchParams.set('project_id', projectId);

    const response = await fetch(url.toString(), {
      headers,
      signal: AbortSignal.timeout(10000),
    });

    if (!response.ok) {
      const text = await response.text();
      throw new Error(`todoist: HTTP ${response.status} – ${text.slice(0, 300)}`);
    }

    const data = (await response.json()) as TodoistApiTask[];

    return {
      provider: 'todoist_deterministic',
      action: 'list',
      tasks: data.map((t) => ({
        task_id: t.id,
        content: t.content,
        project_id: t.project_id,
        due_string: t.due?.string,
        priority: t.priority,
        completed: t.is_completed,
      })),
    };
  }

  if (input.action === 'create') {
    const content = input.task?.content?.trim();
    if (!content) throw new Error('todoist: content is required for create');

    // Plan §6 step 18: POST /tasks
    const response = await fetch(`${TODOIST_BASE}/tasks`, {
      method: 'POST',
      headers,
      body: JSON.stringify({
        content,
        project_id: projectId,
        due_string: input.task?.due_string,
        priority: input.task?.priority ?? 1,
      }),
      signal: AbortSignal.timeout(10000),
    });

    if (!response.ok) {
      const text = await response.text();
      throw new Error(`todoist: HTTP ${response.status} – ${text.slice(0, 300)}`);
    }

    const data = (await response.json()) as TodoistApiTask;
    return {
      provider: 'todoist_deterministic',
      action: 'create',
      task_id: data.id,
      tasks: [
        {
          task_id: data.id,
          content: data.content,
          project_id: data.project_id,
          due_string: data.due?.string,
          priority: data.priority,
          completed: false,
        },
      ],
    };
  }

  if (input.action === 'complete') {
    const taskId = input.task?.task_id?.trim();
    if (!taskId) throw new Error('todoist: task_id is required for complete');

    // Plan §6 step 19: POST /tasks/{id}/close (HTTP 204)
    const response = await fetch(`${TODOIST_BASE}/tasks/${taskId}/close`, {
      method: 'POST',
      headers,
      signal: AbortSignal.timeout(10000),
    });

    if (response.status !== 204) {
      const text = await response.text().catch(() => '');
      throw new Error(
        `todoist: complete failed – expected 204, got HTTP ${response.status} ${text.slice(0, 200)}`
      );
    }

    return {
      provider: 'todoist_deterministic',
      action: 'complete',
      task_id: taskId,
    };
  }

  if (input.action === 'delete') {
    const taskId = input.task?.task_id?.trim();
    if (!taskId) throw new Error('todoist: task_id is required for delete');

    // Plan §6 step 19: DELETE /tasks/{id}
    const response = await fetch(`${TODOIST_BASE}/tasks/${taskId}`, {
      method: 'DELETE',
      headers,
      signal: AbortSignal.timeout(10000),
    });

    if (response.status !== 204) {
      const text = await response.text().catch(() => '');
      throw new Error(
        `todoist: delete failed – expected 204, got HTTP ${response.status} ${text.slice(0, 200)}`
      );
    }

    return {
      provider: 'todoist_deterministic',
      action: 'delete',
      task_id: taskId,
    };
  }

  throw new Error(`todoist: unknown action ${input.action}`);
}

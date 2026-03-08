import type { TaskDecompositionOutput, TaskDescriptor } from './types.js';

function splitRequestIntoSegments(request: string): string[] {
  const normalized = request.replace(/\s+/g, ' ').trim();
  if (normalized === '') {
    return ['handle request'];
  }

  const parts = normalized
    .split(/\b(?:and then|then|and|also)\b/i)
    .map((part) => part.trim())
    .filter((part) => part.length > 0);

  return parts.length > 0 ? parts : [normalized];
}

function assignExecutionOrder(request: string, taskCount: number): 'parallel' | 'sequential' | 'mixed' {
  const normalized = request.toLowerCase();
  if (taskCount <= 1) {
    return 'sequential';
  }
  if (normalized.includes('then') || normalized.includes('after')) {
    return 'sequential';
  }
  if (normalized.includes(' and ') && taskCount > 1) {
    return 'parallel';
  }
  return 'mixed';
}

function validateTaskGraph(tasks: TaskDescriptor[]): void {
  if (tasks.length > 10) {
    throw new Error('TASK_GRAPH_INVALID: max 10 tasks');
  }

  const ids = new Set(tasks.map((task) => task.id));
  const inDegree = new Map<string, number>();
  const adjacency = new Map<string, string[]>();

  for (const task of tasks) {
    inDegree.set(task.id, task.dependencies.length);
    adjacency.set(task.id, []);
  }

  for (const task of tasks) {
    for (const dep of task.dependencies) {
      if (!ids.has(dep)) {
        throw new Error(`TASK_GRAPH_INVALID: unknown dependency ${dep}`);
      }
      const list = adjacency.get(dep);
      if (!list) {
        throw new Error(`TASK_GRAPH_INVALID: missing node ${dep}`);
      }
      list.push(task.id);
    }
  }

  const queue: string[] = [];
  for (const [id, degree] of inDegree.entries()) {
    if (degree === 0) {
      queue.push(id);
    }
  }

  let visited = 0;
  while (queue.length > 0) {
    const current = queue.shift();
    if (!current) {
      break;
    }
    visited += 1;
    const neighbors = adjacency.get(current) ?? [];
    for (const neighbor of neighbors) {
      const nextDegree = (inDegree.get(neighbor) ?? 0) - 1;
      inDegree.set(neighbor, nextDegree);
      if (nextDegree === 0) {
        queue.push(neighbor);
      }
    }
  }

  if (visited !== tasks.length) {
    throw new Error('TASK_GRAPH_INVALID: cycle detected');
  }
}

export function decomposeTask(request: string, skills: string[], requiresDecomposition: boolean): TaskDecompositionOutput {
  const baseSkills = skills.length > 0 ? skills : ['doing-tasks'];
  const segments = requiresDecomposition ? splitRequestIntoSegments(request) : [request || 'handle request'];
  const executionOrder = assignExecutionOrder(request, segments.length);

  const tasks: TaskDescriptor[] = segments.slice(0, 10).map((segment, index) => {
    const dependencies =
      executionOrder === 'sequential' && index > 0
        ? [`t${index}`]
        : [];

    return {
      id: `t${index + 1}`,
      skill_id: baseSkills[index % baseSkills.length],
      input: {
        request_segment: segment
      },
      dependencies,
      priority: index + 1
    };
  });

  validateTaskGraph(tasks);

  return {
    tasks,
    execution_order: executionOrder
  };
}

import type { TaskDecompositionOutput, TaskDescriptor } from './types.js';

const ACTION_VERBS = [
  'add',
  'book',
  'buy',
  'capture',
  'caption',
  'compare',
  'create',
  'describe',
  'draft',
  'email',
  'extract',
  'find',
  'look up',
  'note',
  'ocr',
  'play',
  'remind',
  'reply',
  'save',
  'schedule',
  'search',
  'send',
  'summarize',
  'transcribe',
  'track'
];

function normalizeRequest(request: string): string {
  return request.replace(/\s+/g, ' ').trim();
}

function looksLikeAction(segment: string): boolean {
  const normalized = segment.toLowerCase();
  return ACTION_VERBS.some((verb) => normalized.includes(verb));
}

function splitBySequentialCues(segment: string): string[] {
  return segment
    .split(/\b(?:and then|then|after that|afterwards|next)\b/i)
    .map((part) => part.trim())
    .filter((part) => part.length > 0);
}

function splitByParallelCue(segment: string): string[] {
  if (!segment.toLowerCase().includes(' and ')) {
    return [segment.trim()];
  }

  const parts = segment
    .split(/\band\b/i)
    .map((part) => part.trim())
    .filter((part) => part.length > 0);

  if (parts.length < 2 || !parts.every((part) => looksLikeAction(part))) {
    return [segment.trim()];
  }

  return parts;
}

function splitRequestIntoBatches(request: string): { batches: string[][]; executionOrder: 'parallel' | 'sequential' | 'mixed' } {
  const normalized = normalizeRequest(request);
  if (normalized === '') {
    return {
      batches: [],
      executionOrder: 'sequential'
    };
  }

  const sentenceParts = normalized
    .split(/[.;?!]\s+/)
    .map((part) => part.trim())
    .filter((part) => part.length > 0);

  const batches = sentenceParts
    .flatMap((part) => splitBySequentialCues(part))
    .map((part) => splitByParallelCue(part))
    .filter((batch) => batch.length > 0);

  const hasSequential = batches.length > 1;
  const hasParallel = batches.some((batch) => batch.length > 1);

  return {
    batches: batches.length > 0 ? batches : [[normalized]],
    executionOrder: hasSequential && hasParallel ? 'mixed' : hasSequential ? 'sequential' : hasParallel ? 'parallel' : 'sequential'
  };
}

function validateTaskGraph(tasks: TaskDescriptor[]): void {
  if (tasks.length === 0) {
    throw new Error('TASK_GRAPH_INVALID: request_required');
  }
  if (tasks.length > 10) {
    throw new Error('TASK_GRAPH_INVALID: max 10 tasks');
  }

  const ids = new Set(tasks.map((task) => task.id));
  for (const task of tasks) {
    if (task.goal.trim() === '') {
      throw new Error('TASK_GRAPH_INVALID: empty_task_goal');
    }
    for (const dep of task.dependencies) {
      if (!ids.has(dep)) {
        throw new Error(`TASK_GRAPH_INVALID: unknown dependency ${dep}`);
      }
    }
  }
}

export function decomposeTask(request: string, skills: string[], requiresDecomposition: boolean): TaskDecompositionOutput {
  const normalized = normalizeRequest(request);
  if (normalized === '') {
    throw new Error('TASK_GRAPH_INVALID: request_required');
  }

  const { batches, executionOrder } = requiresDecomposition
    ? splitRequestIntoBatches(normalized)
    : { batches: [[normalized]], executionOrder: 'sequential' as const };

  const baseSkills = skills.length > 0 ? skills : [];
  const tasks: TaskDescriptor[] = [];
  let taskIndex = 0;
  let previousBatchTaskIds: string[] = [];

  for (const batch of batches) {
    const currentBatchTaskIds: string[] = [];
    for (const segment of batch) {
      if (tasks.length >= 10) {
        break;
      }
      taskIndex += 1;
      const taskId = `t${taskIndex}`;
      currentBatchTaskIds.push(taskId);
      tasks.push({
        id: taskId,
        goal: segment,
        intent: 'pending.intent',
        skill_id: baseSkills[taskIndex - 1] ?? baseSkills[0],
        input: {
          request_segment: segment
        },
        dependencies: previousBatchTaskIds,
        priority: taskIndex,
        status: 'planned',
        reasoning: `Segment ${taskIndex} extracted from the original request.`
      });
    }
    if (currentBatchTaskIds.length > 0) {
      previousBatchTaskIds = currentBatchTaskIds;
    }
  }

  validateTaskGraph(tasks);

  return {
    tasks,
    execution_order: executionOrder,
    requires_clarification: tasks.some((task) => !looksLikeAction(task.goal)),
    reasoning: [
      `Decomposed request into ${tasks.length} action segment${tasks.length === 1 ? '' : 's'}.`,
      executionOrder === 'sequential'
        ? 'Execution order is sequential because the request contained ordering cues.'
        : executionOrder === 'parallel'
          ? 'Execution order is parallel because the request contains multiple independent actions.'
          : 'Execution order is mixed because the request contains both sequential and independent actions.'
    ]
  };
}

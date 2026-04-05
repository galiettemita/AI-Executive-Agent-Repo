import type { TaskDecompositionOutput, TaskDescriptor } from './types.js';

const ACTION_VERBS = [
  'add',
  'book',
  'buy',
  'capture',
  'compare',
  'create',
  'draft',
  'email',
  'find',
  'look up',
  'note',
  'play',
  'remind',
  'reply',
  'save',
  'schedule',
  'search',
  'send',
  'summarize',
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

function splitRequestIntoSegments(request: string): { segments: string[]; executionOrder: 'parallel' | 'sequential' | 'mixed' } {
  const normalized = normalizeRequest(request);
  if (normalized === '') {
    return {
      segments: [],
      executionOrder: 'sequential'
    };
  }

  const sentenceParts = normalized
    .split(/[.;?!]\s+/)
    .map((part) => part.trim())
    .filter((part) => part.length > 0);

  const sequentialSegments = sentenceParts.flatMap((part) => splitBySequentialCues(part));
  const finalSegments = sequentialSegments.flatMap((part) => splitByParallelCue(part));

  const hasSequential = /\b(?:and then|then|after that|afterwards|next)\b/i.test(normalized);
  const hasParallel = /\band\b/i.test(normalized) && finalSegments.length > sequentialSegments.length;

  return {
    segments: finalSegments.length > 0 ? finalSegments : [normalized],
    executionOrder: hasSequential && hasParallel ? 'mixed' : hasSequential ? 'sequential' : finalSegments.length > 1 ? 'parallel' : 'sequential'
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

  const { segments, executionOrder } = requiresDecomposition
    ? splitRequestIntoSegments(normalized)
    : { segments: [normalized], executionOrder: 'sequential' as const };

  const baseSkills = skills.length > 0 ? skills : [];

  const tasks: TaskDescriptor[] = segments.slice(0, 10).map((segment, index) => ({
    id: `t${index + 1}`,
    goal: segment,
    intent: 'pending.intent',
    skill_id: baseSkills[index] ?? baseSkills[0],
    input: {
      request_segment: segment
    },
    dependencies: executionOrder === 'sequential' && index > 0 ? [`t${index}`] : [],
    priority: index + 1,
    status: 'planned',
    reasoning: `Segment ${index + 1} extracted from the original request.`
  }));

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

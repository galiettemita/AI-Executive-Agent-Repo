import type { ThingsMacInput, ThingsMacOutput, ThingsMacTodo } from './types.js';

const BASE_TODOS: ThingsMacTodo[] = [
  {
    todo_id: 'things-1',
    title: 'Finalize board memo',
    project: 'Work',
    status: 'open',
    due_date: '2026-03-05T20:00:00.000Z'
  },
  {
    todo_id: 'things-2',
    title: 'Order groceries',
    project: 'Personal',
    status: 'open'
  }
];

function makeTodo(input: ThingsMacInput): ThingsMacTodo {
  return {
    todo_id: input.todo_id ?? `things-${(input.title ?? 'todo').toLowerCase().replace(/[^a-z0-9]+/g, '-').slice(0, 18)}`,
    title: input.title ?? 'Updated Things todo',
    project: input.project ?? 'Inbox',
    status: input.action === 'complete_todo' ? 'completed' : 'open',
    due_date: input.due_date
  };
}

export async function runClient(input: ThingsMacInput): Promise<ThingsMacOutput> {
  if (input.action === 'list_today') {
    return {
      provider: 'things-mac',
      action: input.action,
      todos: BASE_TODOS,
      inbox_count: 3,
      summary: `Loaded ${BASE_TODOS.length} Things todos for today.`
    };
  }

  const todo = makeTodo(input);
  return {
    provider: 'things-mac',
    action: input.action,
    todos: [todo],
    inbox_count: input.action === 'create_todo' ? 4 : 2,
    summary: `Things action ${input.action} applied to ${todo.todo_id}.`
  };
}

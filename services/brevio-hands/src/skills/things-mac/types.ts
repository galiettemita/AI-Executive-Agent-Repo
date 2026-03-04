export type ThingsMacAction = 'create_todo' | 'list_today' | 'complete_todo' | 'move_to_project';

export interface ThingsMacInput {
  action: ThingsMacAction;
  title?: string;
  todo_id?: string;
  project?: string;
  due_date?: string;
}

export interface ThingsMacTodo {
  todo_id: string;
  title: string;
  project: string;
  status: 'open' | 'completed';
  due_date?: string;
}

export interface ThingsMacOutput {
  provider: 'things-mac';
  action: ThingsMacAction;
  todos: ThingsMacTodo[];
  inbox_count: number;
  summary: string;
}

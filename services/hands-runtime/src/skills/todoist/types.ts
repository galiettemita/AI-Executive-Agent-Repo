export type TodoistAction = 'list' | 'create' | 'complete' | 'delete';

export interface TodoistTaskInput {
  task_id?: string;
  content?: string;
  due_string?: string;
  priority?: 1 | 2 | 3 | 4;
}

export interface TodoistInput {
  action: TodoistAction;
  project_id?: string;
  task?: TodoistTaskInput;
}

export interface TodoistTask {
  task_id: string;
  content: string;
  project_id: string;
  due_string?: string;
  priority: number;
  completed: boolean;
}

export interface TodoistOutput {
  provider: 'todoist_deterministic';
  action: TodoistAction;
  task_id?: string;
  tasks?: TodoistTask[];
}

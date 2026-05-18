export type TickTickAction = 'add_task' | 'list_tasks' | 'complete_task' | 'delete_task';

export interface TickTickInput {
  action: TickTickAction;
  task_content?: string;
  task_id?: string;
  project_id?: string;
  due_date?: string;
}

export interface TickTickTask {
  task_id: string;
  content: string;
  project_id: string;
  status: 'open' | 'completed';
  due_date?: string;
}

export interface TickTickOutput {
  provider: 'ticktick';
  action: TickTickAction;
  tasks: TickTickTask[];
  total_tasks: number;
  summary: string;
}

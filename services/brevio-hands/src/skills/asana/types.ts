export type AsanaAction = 'task_list' | 'task_create' | 'task_update';

export interface AsanaInput {
  action: AsanaAction;
  project_id?: string;
  task_id?: string;
  name?: string;
  notes?: string;
  status?: 'todo' | 'in_progress' | 'done';
}

export interface AsanaTask {
  task_id: string;
  name: string;
  project_id: string;
  status: 'todo' | 'in_progress' | 'done';
}

export interface AsanaOutput {
  provider: 'asana';
  action: AsanaAction;
  task_id?: string;
  tasks?: AsanaTask[];
}

export type ClickupAction = 'task_list' | 'task_create' | 'doc_create' | 'time_start' | 'time_stop';

export interface ClickupInput {
  action: ClickupAction;
  list_id?: string;
  task_id?: string;
  title?: string;
  content?: string;
}

export interface ClickupTask {
  task_id: string;
  title: string;
  status: string;
}

export interface ClickupOutput {
  provider: 'clickup-mcp';
  action: ClickupAction;
  task_id?: string;
  doc_id?: string;
  timer_started?: boolean;
  tasks?: ClickupTask[];
}

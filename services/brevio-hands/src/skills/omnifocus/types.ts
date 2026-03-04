export type OmnifocusAction = 'add_task' | 'list_flagged' | 'complete_task' | 'defer_task';

export interface OmnifocusInput {
  action: OmnifocusAction;
  title?: string;
  task_id?: string;
  project?: string;
  defer_until?: string;
}

export interface OmnifocusTask {
  task_id: string;
  title: string;
  project: string;
  status: 'available' | 'completed' | 'deferred';
  defer_until?: string;
}

export interface OmnifocusOutput {
  provider: 'omnifocus';
  action: OmnifocusAction;
  tasks: OmnifocusTask[];
  flagged_count: number;
  summary: string;
}

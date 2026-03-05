export type DoingTasksAction = 'route_task' | 'status_report';

export interface DoingTasksInput {
  action: DoingTasksAction;
  task?: string;
  skill_hint?: string;
}

export interface DoingTasksOutput {
  provider: 'doing-tasks';
  action: DoingTasksAction;
  routed_skill: string;
  execution_plan: string[];
  summary: string;
}

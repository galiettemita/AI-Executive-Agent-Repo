export type PlanMyDayAction = 'build_plan' | 'rebalance_plan';

export type PlanTaskPriority = 'high' | 'medium' | 'low';

export interface PlanMyDayTask {
  title: string;
  duration_minutes: number;
  priority: PlanTaskPriority;
  energy: 'deep' | 'admin';
}

export interface PlanMyDayWindow {
  start_local: string;
  end_local: string;
}

export interface PlanMyDayTimeBlock {
  title: string;
  start_local: string;
  end_local: string;
  source: 'task' | 'break' | 'buffer';
}

export interface PlanMyDayInput {
  action: PlanMyDayAction;
  timezone: string;
  date: string;
  tasks?: PlanMyDayTask[];
  available_windows?: PlanMyDayWindow[];
  disruptions?: string[];
}

export interface PlanMyDayOutput {
  provider: 'plan-my-day';
  action: PlanMyDayAction;
  time_blocks: PlanMyDayTimeBlock[];
  overflow_tasks: string[];
  strategy_notes: string[];
}

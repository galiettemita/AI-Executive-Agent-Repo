export type ProactiveResearchAction = 'monitor_topic' | 'summarize_updates';

export interface ProactiveResearchInput {
  action: ProactiveResearchAction;
  topic?: string;
}

export interface ProactiveResearchOutput {
  provider: 'proactive-research';
  action: ProactiveResearchAction;
  alerts: string[];
  next_check_at: string;
  summary: string;
}

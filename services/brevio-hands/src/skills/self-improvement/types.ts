export type SelfImprovementAction = 'log_lesson' | 'weekly_review';

export interface SelfImprovementInput {
  action: SelfImprovementAction;
  lesson?: string;
  category?: string;
}

export interface SelfImprovementOutput {
  provider: 'self-improvement';
  action: SelfImprovementAction;
  improvements: string[];
  next_steps: string[];
  summary: string;
}

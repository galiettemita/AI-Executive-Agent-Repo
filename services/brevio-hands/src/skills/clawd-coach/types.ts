export type ClawdCoachAction = 'build_plan' | 'log_session';

export interface ClawdCoachInput {
  action: ClawdCoachAction;
  goal?: string;
  weeks?: number;
  session_notes?: string;
}

export interface ClawdCoachOutput {
  provider: 'clawd-coach';
  action: ClawdCoachAction;
  workouts: string[];
  milestones: string[];
  summary: string;
}

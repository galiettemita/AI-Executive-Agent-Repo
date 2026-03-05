export type FocusModeAction = 'start_session' | 'check_in' | 'end_session';

export interface FocusModeInput {
  action: FocusModeAction;
  goal?: string;
  duration_minutes?: number;
  session_id?: string;
  distraction_note?: string;
  completed_tasks?: string[];
}

export interface FocusModeOutput {
  provider: 'focus-mode';
  action: FocusModeAction;
  session_id: string;
  status: 'active' | 'checking_in' | 'completed';
  check_in_schedule: string[];
  next_prompt: string;
  summary?: string;
}

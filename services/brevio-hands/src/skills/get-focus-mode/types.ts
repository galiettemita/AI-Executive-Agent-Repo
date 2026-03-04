export type GetFocusModeAction = 'current_mode' | 'upcoming_schedule';

export interface GetFocusModeInput {
  action: GetFocusModeAction;
  timezone?: string;
}

export interface FocusModeScheduleWindow {
  starts_local: string;
  ends_local: string;
  mode: string;
}

export interface GetFocusModeOutput {
  provider: 'get-focus-mode';
  action: GetFocusModeAction;
  current_mode: string;
  schedule: FocusModeScheduleWindow[];
  summary: string;
}

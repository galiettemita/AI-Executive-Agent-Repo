import type { GetFocusModeInput, GetFocusModeOutput } from './types.js';

const SCHEDULE: GetFocusModeOutput['schedule'] = [
  { starts_local: '09:00', ends_local: '11:30', mode: 'Work' },
  { starts_local: '12:30', ends_local: '13:30', mode: 'Personal' },
  { starts_local: '14:00', ends_local: '18:00', mode: 'Do Not Disturb' }
];

export async function runClient(input: GetFocusModeInput): Promise<GetFocusModeOutput> {
  const current_mode = 'Work';
  return {
    provider: 'get-focus-mode',
    action: input.action,
    current_mode,
    schedule: input.action === 'upcoming_schedule' ? SCHEDULE : [],
    summary:
      input.action === 'upcoming_schedule'
        ? `Current mode is ${current_mode}; ${SCHEDULE.length} upcoming focus window(s) available.`
        : `Current Focus mode is ${current_mode}.`
  };
}

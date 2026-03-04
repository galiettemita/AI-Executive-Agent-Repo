import type { FocusModeInput, FocusModeOutput } from './types.js';

function toSchedule(durationMinutes: number): string[] {
  const checkpoints = Math.max(1, Math.floor(durationMinutes / 25));
  const schedule: string[] = [];
  let minute = 0;
  for (let i = 0; i < checkpoints; i += 1) {
    const hour = Math.floor(minute / 60);
    const remainder = minute % 60;
    schedule.push(`${String(hour).padStart(2, '0')}:${String(remainder).padStart(2, '0')}`);
    minute += 25;
  }
  return schedule;
}

function buildSessionId(goal: string): string {
  return `focus_${goal.toLowerCase().replace(/[^a-z0-9]+/g, '_').replace(/^_|_$/g, '').slice(0, 18)}_001`;
}

export async function runClient(input: FocusModeInput): Promise<FocusModeOutput> {
  if (input.action === 'start_session') {
    const session_id = buildSessionId(input.goal ?? 'session');
    return {
      provider: 'focus-mode',
      action: input.action,
      session_id,
      status: 'active',
      check_in_schedule: toSchedule(input.duration_minutes ?? 25),
      next_prompt: `Focus now on: ${input.goal}. I will check in at the next interval.`
    };
  }

  if (input.action === 'check_in') {
    return {
      provider: 'focus-mode',
      action: input.action,
      session_id: input.session_id ?? 'focus_unknown_001',
      status: 'checking_in',
      check_in_schedule: ['00:25'],
      next_prompt: input.distraction_note
        ? `Acknowledge distraction: ${input.distraction_note}. Return to the primary goal.`
        : 'Stay on the current objective and close one concrete sub-task now.'
    };
  }

  const completed = (input.completed_tasks ?? []).join(', ');
  return {
    provider: 'focus-mode',
    action: input.action,
    session_id: input.session_id ?? 'focus_unknown_001',
    status: 'completed',
    check_in_schedule: [],
    next_prompt: 'Session complete. Decide the next highest-impact task before context switching.',
    summary: completed
      ? `Completed tasks: ${completed}.`
      : 'Session completed. No explicit completed-task list was provided.'
  };
}

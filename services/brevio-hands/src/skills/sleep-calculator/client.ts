import type {
  SleepCalculatorAction,
  SleepCalculatorInput,
  SleepCalculatorOutput,
  SleepRecommendation
} from './types.js';

function toMinutes(clock: string): number {
  const [hours, minutes] = clock.split(':').map((value) => Number(value));
  return hours * 60 + minutes;
}

function toClock(minutesValue: number): string {
  let normalized = minutesValue % (24 * 60);
  if (normalized < 0) {
    normalized += 24 * 60;
  }
  const hour = Math.floor(normalized / 60);
  const minute = normalized % 60;
  return `${String(hour).padStart(2, '0')}:${String(minute).padStart(2, '0')}`;
}

function buildRecommendations(
  action: SleepCalculatorAction,
  anchorTime: string,
  cycleMinutes: number
): SleepRecommendation[] {
  const anchorMinutes = toMinutes(anchorTime);
  const cycles = [6, 5, 4];

  return cycles.map((sleepCycles) => {
    const delta = sleepCycles * cycleMinutes + 15;
    const target = action === 'bedtime_from_wake' ? anchorMinutes - delta : anchorMinutes + delta;
    return {
      target_time_local: toClock(target),
      sleep_cycles: sleepCycles,
      hours_in_bed: Number(((sleepCycles * cycleMinutes + 15) / 60).toFixed(1))
    };
  });
}

export async function runClient(input: SleepCalculatorInput): Promise<SleepCalculatorOutput> {
  const cycle = input.sleep_cycle_minutes ?? 90;
  const anchorTime = input.action === 'bedtime_from_wake' ? input.wake_time_local : input.bedtime_local;
  const recommendations = buildRecommendations(input.action, anchorTime ?? '07:00', cycle);

  return {
    provider: 'sleep-calculator',
    action: input.action,
    recommendations,
    summary: `Generated ${recommendations.length} sleep timing option(s) using ${cycle}-minute cycle assumptions.`
  };
}

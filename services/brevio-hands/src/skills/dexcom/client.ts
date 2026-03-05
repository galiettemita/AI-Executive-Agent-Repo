import type { DexcomInput, DexcomOutput, DexcomReading } from './types.js';

function stateFor(value: number): DexcomReading['state'] {
  if (value < 70) {
    return 'low';
  }
  if (value > 180) {
    return 'high';
  }
  return 'in_range';
}

export async function runClient(input: DexcomInput): Promise<DexcomOutput> {
  const readings: DexcomReading[] = [
    {
      timestamp: '2026-03-04T08:00:00.000Z',
      value_mg_dl: 112,
      trend: 'steady',
      state: stateFor(112)
    },
    {
      timestamp: '2026-03-04T08:05:00.000Z',
      value_mg_dl: 118,
      trend: 'rising',
      state: stateFor(118)
    },
    {
      timestamp: '2026-03-04T08:10:00.000Z',
      value_mg_dl: 124,
      trend: 'rising',
      state: stateFor(124)
    }
  ];

  const alerts = readings
    .filter((reading) => reading.state !== 'in_range')
    .map((reading) => `${reading.timestamp}: glucose ${reading.state} at ${reading.value_mg_dl} mg/dL`);

  return {
    provider: 'dexcom',
    action: input.action,
    readings,
    alerts,
    summary:
      alerts.length === 0
        ? 'Glucose remained in range in sampled interval.'
        : `Detected ${alerts.length} glucose alert(s) in sampled interval.`
  };
}

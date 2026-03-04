import type {
  WithingsHealthInput,
  WithingsHealthOutput,
  WithingsMeasurement,
  WithingsMeasureType
} from './types.js';

function measureUnit(type: WithingsMeasureType): string {
  switch (type) {
    case 'weight':
      return 'kg';
    case 'body_fat_pct':
      return '%';
    case 'muscle_mass_kg':
      return 'kg';
    case 'heart_rate_bpm':
      return 'bpm';
    default:
      return 'unit';
  }
}

function baseValue(type: WithingsMeasureType): number {
  switch (type) {
    case 'weight':
      return 78.4;
    case 'body_fat_pct':
      return 18.2;
    case 'muscle_mass_kg':
      return 34.1;
    case 'heart_rate_bpm':
      return 62;
    default:
      return 0;
  }
}

function buildMeasurements(type: WithingsMeasureType): WithingsMeasurement[] {
  const unit = measureUnit(type);
  const baseline = baseValue(type);
  return [
    {
      recorded_at: '2026-03-02T07:05:00.000Z',
      measure_type: type,
      value: Number((baseline + 0.2).toFixed(2)),
      unit
    },
    {
      recorded_at: '2026-03-03T07:05:00.000Z',
      measure_type: type,
      value: Number((baseline + 0.1).toFixed(2)),
      unit
    },
    {
      recorded_at: '2026-03-04T07:05:00.000Z',
      measure_type: type,
      value: Number(baseline.toFixed(2)),
      unit
    }
  ];
}

export async function runClient(input: WithingsHealthInput): Promise<WithingsHealthOutput> {
  const measure_type = input.measure_type ?? 'weight';
  const measurements = buildMeasurements(measure_type);
  const trend: WithingsHealthOutput['trend'] =
    measurements[2].value > measurements[0].value
      ? 'up'
      : measurements[2].value < measurements[0].value
        ? 'down'
        : 'stable';

  return {
    provider: 'withings-health',
    action: input.action,
    measure_type,
    measurements,
    trend,
    summary: `Withings ${measure_type} trend is ${trend} over the last three check-ins.`
  };
}

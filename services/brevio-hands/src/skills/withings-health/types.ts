export type WithingsHealthAction = 'get_measurements' | 'trend_summary';

export type WithingsMeasureType = 'weight' | 'body_fat_pct' | 'muscle_mass_kg' | 'heart_rate_bpm';

export interface WithingsHealthInput {
  action: WithingsHealthAction;
  measure_type?: WithingsMeasureType;
  start_date?: string;
  end_date?: string;
}

export interface WithingsMeasurement {
  recorded_at: string;
  measure_type: WithingsMeasureType;
  value: number;
  unit: string;
}

export interface WithingsHealthOutput {
  provider: 'withings-health';
  action: WithingsHealthAction;
  measure_type: WithingsMeasureType;
  measurements: WithingsMeasurement[];
  trend: 'up' | 'down' | 'stable';
  summary: string;
}

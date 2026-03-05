export type SleepCalculatorAction = 'bedtime_from_wake' | 'wake_from_bedtime';

export interface SleepCalculatorInput {
  action: SleepCalculatorAction;
  wake_time_local?: string;
  bedtime_local?: string;
  sleep_cycle_minutes?: number;
}

export interface SleepRecommendation {
  target_time_local: string;
  sleep_cycles: number;
  hours_in_bed: number;
}

export interface SleepCalculatorOutput {
  provider: 'sleep-calculator';
  action: SleepCalculatorAction;
  recommendations: SleepRecommendation[];
  summary: string;
}

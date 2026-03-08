export type DexcomAction = 'glucose_readings' | 'trend_alerts';

export interface DexcomInput {
  action: DexcomAction;
  start_time?: string;
  end_time?: string;
  minutes?: number;
}

export interface DexcomReading {
  timestamp: string;
  value_mg_dl: number;
  trend: 'rising' | 'falling' | 'steady';
  state: 'low' | 'in_range' | 'high';
}

export interface DexcomOutput {
  provider: 'dexcom';
  action: DexcomAction;
  readings: DexcomReading[];
  alerts: string[];
  summary: string;
}

export type HealthkitSyncAppleAction = 'sync_steps' | 'sync_sleep' | 'sync_heart_rate' | 'sync_all';

export interface HealthkitSyncAppleInput {
  action: HealthkitSyncAppleAction;
  start_date?: string;
  end_date?: string;
  days?: number;
}

export interface HealthkitMetricSnapshot {
  metric: 'steps' | 'sleep_hours' | 'heart_rate_bpm';
  value: number;
  recorded_at: string;
}

export interface HealthkitSyncAppleOutput {
  provider: 'healthkit-sync-apple';
  action: HealthkitSyncAppleAction;
  snapshots: HealthkitMetricSnapshot[];
  synced_metric_count: number;
  summary: string;
}

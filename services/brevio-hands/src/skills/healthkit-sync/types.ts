export type HealthkitSyncAction = 'sync_steps' | 'sync_sleep' | 'sync_heart_rate' | 'sync_all';

export interface HealthkitSyncInput {
  action: HealthkitSyncAction;
  start_date?: string;
  end_date?: string;
  days?: number;
}

export interface HealthkitSyncOutput {
  provider: 'healthkit-sync';
  action: HealthkitSyncAction;
  alias_target: 'healthkit-sync-apple';
  deprecated_alias: true;
  forwarded: boolean;
  summary: string;
}

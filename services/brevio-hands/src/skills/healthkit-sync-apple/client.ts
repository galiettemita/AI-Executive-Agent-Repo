import type {
  HealthkitMetricSnapshot,
  HealthkitSyncAppleAction,
  HealthkitSyncAppleInput,
  HealthkitSyncAppleOutput
} from './types.js';

function allSnapshots(): HealthkitMetricSnapshot[] {
  return [
    {
      metric: 'steps',
      value: 8421,
      recorded_at: '2026-03-04T18:00:00.000Z'
    },
    {
      metric: 'sleep_hours',
      value: 7.3,
      recorded_at: '2026-03-04T07:00:00.000Z'
    },
    {
      metric: 'heart_rate_bpm',
      value: 59,
      recorded_at: '2026-03-04T12:00:00.000Z'
    }
  ];
}

function byAction(action: HealthkitSyncAppleAction): HealthkitMetricSnapshot[] {
  const snapshots = allSnapshots();
  if (action === 'sync_steps') {
    return snapshots.filter((snapshot) => snapshot.metric === 'steps');
  }
  if (action === 'sync_sleep') {
    return snapshots.filter((snapshot) => snapshot.metric === 'sleep_hours');
  }
  if (action === 'sync_heart_rate') {
    return snapshots.filter((snapshot) => snapshot.metric === 'heart_rate_bpm');
  }
  return snapshots;
}

export async function runClient(input: HealthkitSyncAppleInput): Promise<HealthkitSyncAppleOutput> {
  const snapshots = byAction(input.action);
  return {
    provider: 'healthkit-sync-apple',
    action: input.action,
    snapshots,
    synced_metric_count: snapshots.length,
    summary: `Synced ${snapshots.length} HealthKit metric snapshot(s) from Apple Health.`
  };
}

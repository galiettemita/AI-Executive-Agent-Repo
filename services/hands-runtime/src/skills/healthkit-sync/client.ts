import type { HealthkitSyncInput, HealthkitSyncOutput } from './types.js';

export async function runClient(input: HealthkitSyncInput): Promise<HealthkitSyncOutput> {
  return {
    provider: 'healthkit-sync',
    action: input.action,
    alias_target: 'healthkit-sync-apple',
    deprecated_alias: true,
    forwarded: true,
    summary:
      'Alias skill invoked. Request should be routed to healthkit-sync-apple for canonical HealthKit processing.'
  };
}

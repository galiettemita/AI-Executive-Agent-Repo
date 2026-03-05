import type { RokuInput, RokuOutput } from './types.js';

export async function runClient(input: RokuInput): Promise<RokuOutput> {
  return {
    provider: 'roku',
    action: input.action,
    device_id: input.device_id ?? 'roku-living-room',
    current_app: input.app_id ?? 'home',
    power_state: 'on',
    summary: `Roku action ${input.action} completed on ${input.device_id ?? 'roku-living-room'}.`
  };
}

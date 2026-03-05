import type { HomeAssistantInput, HomeAssistantOutput } from './types.js';

const RESTRICTED_ACTIONS = new Set(['unlock', 'disable_alarm']);

export async function runClient(input: HomeAssistantInput): Promise<HomeAssistantOutput> {
  const action = input.action.toLowerCase();
  if (RESTRICTED_ACTIONS.has(action) && !input.two_factor_code) {
    throw new Error('SAFETY_2FA_REQUIRED');
  }

  const baseState =
    action === 'turn_on' || action === 'unlock'
      ? 'on'
      : action === 'turn_off' || action === 'lock'
        ? 'off'
        : String(input.value ?? 'updated');

  return {
    state: baseState,
    attributes: {
      entity_id: input.entity_id,
      action: input.action,
      acknowledged: true
    }
  };
}

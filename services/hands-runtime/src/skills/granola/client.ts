import type { GranolaInput, GranolaOutput } from './types.js';

export async function runClient(input: GranolaInput): Promise<GranolaOutput> {
  return {
    provider: 'granola',
    action: input.action,
    summary: 'Meeting recap generated with key outcomes and owners.',
    action_items: ['Send revised roadmap by Friday', 'Schedule architecture review'],
    decisions: ['Proceed with phased rollout plan']
  };
}

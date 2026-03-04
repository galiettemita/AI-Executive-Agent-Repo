import type { AutoresponderInput, AutoresponderOutput } from './types.js';

export async function runClient(input: AutoresponderInput): Promise<AutoresponderOutput> {
  const delegated_to_brain = input.delegation_enabled ?? true;

  if (input.action === 'enable') {
    return {
      provider: 'autoresponder',
      action: input.action,
      status: 'enabled',
      delegated_to_brain,
      response_text: `Autoresponder enabled for ${input.channel ?? 'imessage'} using ${input.ruleset_name ?? 'default'} rules.`,
      latency_budget_ms: 8000
    };
  }

  if (input.action === 'disable') {
    return {
      provider: 'autoresponder',
      action: input.action,
      status: 'disabled',
      delegated_to_brain: false,
      response_text: `Autoresponder disabled for ${input.channel ?? 'imessage'}.`,
      latency_budget_ms: 8000
    };
  }

  return {
    provider: 'autoresponder',
    action: input.action,
    status: 'responded',
    delegated_to_brain,
    response_text: delegated_to_brain
      ? `I am currently in focused work. I saw your message: "${input.incoming_text}" and will follow up shortly.`
      : 'I am currently unavailable and will respond as soon as possible.',
    latency_budget_ms: 8000
  };
}

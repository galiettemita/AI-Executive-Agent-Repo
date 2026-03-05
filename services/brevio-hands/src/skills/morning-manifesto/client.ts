import type { MorningManifestoInput, MorningManifestoOutput } from './types.js';

function normalizeActionItems(input: MorningManifestoInput): string[] {
  const goals = input.goals ?? [];
  const blockers = input.blockers ?? [];
  const items = goals.map((goal, index) => `Prioritize #${index + 1}: ${goal}`);
  for (const blocker of blockers) {
    items.push(`Mitigate blocker: ${blocker}`);
  }
  return items.slice(0, 20);
}

export async function runClient(input: MorningManifestoInput): Promise<MorningManifestoOutput> {
  const tone = input.tone ?? 'direct';
  const action_items = normalizeActionItems(input);
  const sync_targets = input.sync_targets ?? [];
  const gratitude = (input.gratitude ?? []).join(', ');

  const manifesto =
    tone === 'supportive'
      ? `You have a focused day ahead. Lead with clarity, execute your top commitments, and keep momentum through friction. ${gratitude ? `Carry gratitude for: ${gratitude}.` : ''}`
      : `Today is execution day: protect your highest-impact goals first and cut low-value drift early. ${gratitude ? `Remember: ${gratitude}.` : ''}`;

  return {
    provider: 'morning-manifesto',
    action: input.action,
    manifesto,
    affirmations: [
      'I execute the most important work before reacting to noise.',
      'I communicate clearly and make decisions deliberately.',
      'I finish the day with fewer open loops than I started.'
    ],
    action_items,
    sync_targets
  };
}

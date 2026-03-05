import type { ProsConsInput, ProsConsOutput } from './types.js';

export async function runClient(input: ProsConsInput): Promise<ProsConsOutput> {
  const options = (input.options ?? []).map((option, index) => ({
    option,
    pros: [`Advantage ${index + 1} for ${option}`],
    cons: [`Tradeoff ${index + 1} for ${option}`],
    score: 70 - index * 5
  }));

  return {
    provider: 'pros-cons',
    action: 'evaluate_decision',
    options,
    recommendation: options[0]?.option ?? 'No recommendation',
    summary: `Scored ${options.length} options for decision "${input.decision}".`
  };
}

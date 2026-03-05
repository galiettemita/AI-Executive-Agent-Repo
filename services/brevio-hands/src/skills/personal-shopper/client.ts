import type {
  PersonalShopperCandidate,
  PersonalShopperInput,
  PersonalShopperOutput
} from './types.js';

function rankCandidates(input: PersonalShopperInput): PersonalShopperCandidate[] {
  const budget = input.budget_cents ?? Number.MAX_SAFE_INTEGER;

  const baseCandidates =
    input.candidates?.map((candidate) => {
      const budgetPenalty = candidate.price_cents > budget ? 20 : 0;
      const score = Math.max(1, 90 - budgetPenalty - Math.floor(candidate.price_cents / 5000));
      return {
        name: candidate.name,
        price_cents: candidate.price_cents,
        score,
        pros: ['Strong value for target use case', 'Good reliability trend'],
        cons: candidate.price_cents > budget ? ['Over target budget'] : ['Fewer premium extras'],
        buy_url: `https://shop.example.com/${encodeURIComponent(candidate.name.toLowerCase().replace(/\s+/g, '-'))}`
      } satisfies PersonalShopperCandidate;
    }) ?? [];

  return baseCandidates.sort((left, right) => right.score - left.score).slice(0, 10);
}

export async function runClient(input: PersonalShopperInput): Promise<PersonalShopperOutput> {
  const ranked_candidates =
    input.action === 'research_product'
      ? [
          {
            name: `${input.query} Option A`,
            price_cents: Math.min(input.budget_cents ?? 120000, 119999),
            score: 88,
            pros: ['Strong review consistency', 'Fits likely budget'],
            cons: ['Limited color options'],
            buy_url: 'https://shop.example.com/research-option-a'
          },
          {
            name: `${input.query} Option B`,
            price_cents: Math.min((input.budget_cents ?? 120000) + 15000, 140000),
            score: 79,
            pros: ['Premium build quality'],
            cons: ['Higher price point'],
            buy_url: 'https://shop.example.com/research-option-b'
          }
        ]
      : rankCandidates(input);

  const best = ranked_candidates[0];

  return {
    provider: 'personal-shopper',
    action: input.action,
    summary: `Evaluated ${ranked_candidates.length} candidates for ${input.query ?? 'requested purchase'} using budget and preference constraints.`,
    ranked_candidates,
    recommendation: best
      ? `${best.name} is the top recommendation with score ${best.score} at $${(best.price_cents / 100).toFixed(2)}.`
      : 'No viable candidates were found with current constraints.',
    purchase_steps:
      input.action === 'purchase_plan'
        ? [
            'Confirm preferred vendor and warranty terms.',
            'Verify total including shipping and tax before checkout.',
            'Place order and store receipt for follow-up tracking.'
          ]
        : undefined
  };
}

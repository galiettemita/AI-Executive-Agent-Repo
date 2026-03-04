import type { CardOptimizerInput, CardOptimizerOutput } from './types.js';

const DEFAULT_CARDS: NonNullable<CardOptimizerInput['available_cards']> = [
  { card_name: 'Brevio Gold', reward_type: 'points', category_bonus_pct: 4.0, base_pct: 1.0 },
  { card_name: 'Brevio Travel', reward_type: 'miles', category_bonus_pct: 3.0, base_pct: 1.5 },
  { card_name: 'Brevio Cash', reward_type: 'cashback', category_bonus_pct: 2.5, base_pct: 2.0 }
];

function rewardCents(amountCents: number, percent: number): number {
  return Math.round((amountCents * percent) / 100);
}

export async function runClient(input: CardOptimizerInput): Promise<CardOptimizerOutput> {
  const cards = input.available_cards ?? DEFAULT_CARDS;
  const amount = input.amount_cents ?? 0;

  const ranked = cards
    .map((card) => ({
      card_name: card.card_name,
      estimated_reward_cents: rewardCents(amount, card.category_bonus_pct || card.base_pct)
    }))
    .sort((left, right) => right.estimated_reward_cents - left.estimated_reward_cents);

  const top = ranked[0];

  return {
    provider: 'card-optimizer',
    action: input.action,
    recommended_card: top.card_name,
    estimated_reward_cents: top.estimated_reward_cents,
    alternatives: ranked.slice(1),
    rationale: `${top.card_name} yields the highest projected rewards for ${input.purchase_category} at this spend amount.`
  };
}

export type CardOptimizerAction = 'recommend_card' | 'category_strategy';

export interface CardOptimizerInput {
  action: CardOptimizerAction;
  purchase_category?: string;
  amount_cents?: number;
  available_cards?: Array<{
    card_name: string;
    reward_type: 'cashback' | 'points' | 'miles';
    category_bonus_pct: number;
    base_pct: number;
  }>;
}

export interface CardOptimizerOutput {
  provider: 'card-optimizer';
  action: CardOptimizerAction;
  recommended_card: string;
  estimated_reward_cents: number;
  alternatives: Array<{ card_name: string; estimated_reward_cents: number }>;
  rationale: string;
}

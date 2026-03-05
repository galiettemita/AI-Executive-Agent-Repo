export type PersonalShopperAction = 'research_product' | 'rank_options' | 'purchase_plan';

export interface PersonalShopperCandidate {
  name: string;
  price_cents: number;
  score: number;
  pros: string[];
  cons: string[];
  buy_url?: string;
}

export interface PersonalShopperInput {
  action: PersonalShopperAction;
  query?: string;
  budget_cents?: number;
  constraints?: string[];
  candidates?: Array<{ name: string; price_cents: number; features?: string[] }>;
}

export interface PersonalShopperOutput {
  provider: 'personal-shopper';
  action: PersonalShopperAction;
  summary: string;
  ranked_candidates: PersonalShopperCandidate[];
  recommendation: string;
  purchase_steps?: string[];
}

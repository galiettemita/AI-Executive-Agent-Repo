export type MarketplaceAction = 'evaluate_listing' | 'compare_prices' | 'draft_listing';

export interface MarketplaceInput {
  action: MarketplaceAction;
  title?: string;
  listing_url?: string;
  condition?: 'new' | 'like_new' | 'good' | 'fair';
  asking_price_cents?: number;
  comparable_prices_cents?: number[];
}

export interface MarketplaceOutput {
  provider: 'marketplace';
  action: MarketplaceAction;
  fair_price_cents: number;
  confidence: number;
  scam_risk: 'low' | 'medium' | 'high';
  summary: string;
  draft_listing_copy?: string;
}

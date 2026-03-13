import type { MarketplaceInput, MarketplaceOutput } from './types.js';

function median(values: number[]): number {
  const sorted = [...values].sort((left, right) => left - right);
  const mid = Math.floor(sorted.length / 2);
  if (sorted.length % 2 === 0) {
    return Math.round(((sorted[mid - 1] as number) + (sorted[mid] as number)) / 2);
  }
  return sorted[mid] as number;
}

function conditionMultiplier(condition: MarketplaceInput['condition']): number {
  switch (condition) {
    case 'new':
      return 1;
    case 'like_new':
      return 0.9;
    case 'good':
      return 0.75;
    case 'fair':
      return 0.55;
    default:
      return 0.7;
  }
}

function calculateFairPrice(input: MarketplaceInput): number {
  if (input.comparable_prices_cents?.length) {
    return median(input.comparable_prices_cents);
  }

  const anchor = input.asking_price_cents ?? 10000;
  return Math.round(anchor * conditionMultiplier(input.condition));
}

export async function runClient(input: MarketplaceInput): Promise<MarketplaceOutput> {
  const fairPrice = calculateFairPrice(input);
  const askingPrice = input.asking_price_cents ?? fairPrice;
  const deviationPct = Math.abs(askingPrice - fairPrice) / fairPrice;

  const scamRisk: MarketplaceOutput['scam_risk'] =
    deviationPct > 0.5 ? 'high' : deviationPct > 0.25 ? 'medium' : 'low';

  const baseOutput: MarketplaceOutput = {
    provider: 'marketplace',
    action: input.action,
    fair_price_cents: fairPrice,
    confidence: input.comparable_prices_cents?.length ? 0.86 : 0.72,
    scam_risk: scamRisk,
    summary: `Estimated fair price is $${(fairPrice / 100).toFixed(2)} for ${input.title ?? 'this item'} given provided market context.`
  };

  if (input.action === 'draft_listing') {
    return {
      ...baseOutput,
      draft_listing_copy: `${input.title} in ${input.condition ?? 'good'} condition. Priced competitively at $${(
        askingPrice / 100
      ).toFixed(2)}. Pickup available and open to reasonable offers.`
    };
  }

  return baseOutput;
}

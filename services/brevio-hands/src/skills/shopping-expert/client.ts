import type { ShoppingExpertInput, ShoppingExpertOutput, ShoppingResultItem } from './types.js';

const CATALOG: ShoppingResultItem[] = [
  {
    title: 'Nimbus Run Pro Shoes',
    price: 89,
    url: 'https://example.com/nimbus-run-pro-shoes',
    rating: 4.6,
    store: 'RunHub'
  },
  {
    title: 'AeroLite Daily Trainer',
    price: 74,
    url: 'https://example.com/aerolite-daily-trainer',
    rating: 4.4,
    store: 'FitDepot'
  },
  {
    title: 'Summit Trail Runner GTX',
    price: 129,
    url: 'https://example.com/summit-trail-runner-gtx',
    rating: 4.7,
    store: 'PeakWorks'
  },
  {
    title: 'Pulse Knit Recovery Sneaker',
    price: 59,
    url: 'https://example.com/pulse-knit-recovery-sneaker',
    rating: 4.2,
    store: 'MoveMart'
  },
  {
    title: 'Stride Flex Carbon Racer',
    price: 179,
    url: 'https://example.com/stride-flex-carbon-racer',
    rating: 4.8,
    store: 'RunHub'
  },
  {
    title: 'City Walk Comfort Low',
    price: 49,
    url: 'https://example.com/city-walk-comfort-low',
    rating: 4.1,
    store: 'UrbanStep'
  }
];

function scoreItem(item: ShoppingResultItem, terms: string[]): number {
  const haystack = `${item.title} ${item.store}`.toLowerCase();
  let score = item.rating;
  for (const term of terms) {
    if (haystack.includes(term)) {
      score += 1.5;
    }
  }
  return score;
}

export async function runClient(input: ShoppingExpertInput): Promise<ShoppingExpertOutput> {
  const terms = input.query
    .toLowerCase()
    .split(/\s+/)
    .map((term) => term.trim())
    .filter((term) => term.length > 1);

  const categoryTerm = input.category?.toLowerCase();
  const limit = input.limit ?? 5;

  const filtered = CATALOG.filter((item) => {
    if (typeof input.max_price === 'number' && item.price > input.max_price) {
      return false;
    }

    if (categoryTerm) {
      const target = `${item.title} ${item.store}`.toLowerCase();
      if (!target.includes(categoryTerm)) {
        return false;
      }
    }

    if (terms.length === 0) {
      return true;
    }

    const target = `${item.title} ${item.store}`.toLowerCase();
    return terms.some((term) => target.includes(term));
  });

  const ranked = filtered
    .map((item) => ({
      item,
      score: scoreItem(item, terms)
    }))
    .sort((a, b) => b.score - a.score)
    .slice(0, limit)
    .map((entry) => entry.item);

  return {
    provider: 'mock_catalog',
    results: ranked
  };
}

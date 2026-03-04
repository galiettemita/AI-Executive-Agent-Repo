import type { SpotsInput, SpotsOutput, SpotsResult } from './types.js';

const SPOTS_RESULTS: SpotsResult[] = [
  {
    name: 'Atlas Coffee Roasters',
    address: '100 Market St, Austin, TX',
    category: 'cafe',
    lat: 30.2665,
    lng: -97.7422
  },
  {
    name: 'Beacon Thai Kitchen',
    address: '210 Lake St, Austin, TX',
    category: 'restaurant',
    lat: 30.2701,
    lng: -97.7511
  },
  {
    name: 'Summit Fitness Club',
    address: '45 Cedar St, Austin, TX',
    category: 'gym',
    lat: 30.2728,
    lng: -97.7388
  },
  {
    name: 'Northside Book Hall',
    address: '550 Burnet Rd, Austin, TX',
    category: 'bookstore',
    lat: 30.3118,
    lng: -97.7493
  }
];

export async function runClient(input: SpotsInput): Promise<SpotsOutput> {
  const terms = input.query.toLowerCase().split(/\s+/u).filter((term) => term.length > 1);
  const gridDensity = input.grid_density ?? 'medium';
  const maxResults = input.max_results ?? 30;

  const matches = SPOTS_RESULTS.filter((item) => {
    const haystack = `${item.name} ${item.category} ${item.address}`.toLowerCase();
    return terms.some((term) => haystack.includes(term));
  }).slice(0, maxResults);

  return {
    provider: 'spots',
    grid_density: gridDensity,
    results: matches
  };
}

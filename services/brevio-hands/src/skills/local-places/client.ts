import type { LocalPlacesInput, LocalPlacesOutput, LocalPlacesResult } from './types.js';

const LOCAL_RESULTS: LocalPlacesResult[] = [
  {
    name: 'Cornerstone Coffee',
    address: '12 Main St, Austin, TX',
    distance_km: 0.9,
    category: 'cafe'
  },
  {
    name: 'Beacon Grocery',
    address: '55 River Rd, Austin, TX',
    distance_km: 1.4,
    category: 'grocery'
  },
  {
    name: 'Northside Pharmacy',
    address: '77 Cedar Ave, Austin, TX',
    distance_km: 2.6,
    category: 'pharmacy'
  }
];

export async function runClient(input: LocalPlacesInput): Promise<LocalPlacesOutput> {
  const terms = input.query.toLowerCase().split(/\s+/u).filter((term) => term.length > 1);
  const maxResults = input.max_results ?? 5;
  const radiusKm = input.radius_km ?? 10;

  const results = LOCAL_RESULTS.filter((item) => {
    const haystack = `${item.name} ${item.category} ${item.address}`.toLowerCase();
    if (!terms.some((term) => haystack.includes(term))) {
      return false;
    }
    return item.distance_km <= radiusKm;
  }).slice(0, maxResults);

  return {
    provider: 'local-places',
    results
  };
}

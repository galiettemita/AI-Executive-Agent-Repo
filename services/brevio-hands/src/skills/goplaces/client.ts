import type { GoPlacesInput, GoPlacesOutput, GoPlacesResult } from './types.js';

const PLACES: GoPlacesResult[] = [
  {
    place_id: 'place_espresso_lane',
    name: 'Espresso Lane',
    formatted_address: '101 Congress Ave, Austin, TX',
    rating: 4.7,
    open_now: true
  },
  {
    place_id: 'place_boardroom_bistro',
    name: 'Boardroom Bistro',
    formatted_address: '222 West 6th St, Austin, TX',
    rating: 4.5,
    open_now: false
  },
  {
    place_id: 'place_harbor_noodle',
    name: 'Harbor Noodle House',
    formatted_address: '380 Lakeshore Dr, Austin, TX',
    rating: 4.6,
    open_now: true
  }
];

export async function runClient(input: GoPlacesInput): Promise<GoPlacesOutput> {
  const terms = input.query.toLowerCase().split(/\s+/u).filter((term) => term.length > 1);
  const maxResults = input.max_results ?? 5;

  const results = PLACES.filter((item) => {
    const haystack = `${item.name} ${item.formatted_address}`.toLowerCase();
    const termMatch = terms.some((term) => haystack.includes(term));
    if (!termMatch) {
      return false;
    }
    if (typeof input.open_now === 'boolean' && item.open_now !== input.open_now) {
      return false;
    }
    return true;
  }).slice(0, maxResults);

  return {
    provider: 'goplaces',
    results
  };
}

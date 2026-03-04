import type { TMDBInput, TMDBOutput, TMDBResult } from './types.js';

const CATALOG: TMDBResult[] = [
  {
    title: 'Signal Window',
    year: 2025,
    rating: 8.2,
    overview: 'A strategy lead uncovers a global decision anomaly.',
    streaming: 'Prime Video'
  },
  {
    title: 'Quarterly',
    year: 2024,
    rating: 7.8,
    overview: 'A workplace drama about rebuilding a company operating system.',
    streaming: 'Netflix'
  },
  {
    title: 'Zero Inbox',
    year: 2023,
    rating: 7.4,
    overview: 'A fast-paced series about comms overload and delegation.',
    streaming: 'Hulu'
  }
];

export async function runClient(input: TMDBInput): Promise<TMDBOutput> {
  const query = input.query?.toLowerCase();
  const genre = input.genre?.toLowerCase();

  const results = CATALOG.filter((item) => {
    const target = `${item.title} ${item.overview}`.toLowerCase();
    if (query && !target.includes(query)) {
      return false;
    }
    if (genre && !target.includes(genre)) {
      return false;
    }
    return true;
  });

  return {
    provider: 'tmdb',
    results
  };
}

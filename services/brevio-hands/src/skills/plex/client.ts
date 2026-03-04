import type { PlexInput, PlexMediaItem, PlexOutput } from './types.js';

const LIBRARY: PlexMediaItem[] = [
  {
    id: 'plex_movie_001',
    title: 'Quarterly Strategy',
    type: 'movie',
    year: 2024
  },
  {
    id: 'plex_episode_020',
    title: 'Ops Weekly Episode 20',
    type: 'episode',
    year: 2026
  }
];

export async function runClient(input: PlexInput): Promise<PlexOutput> {
  if (input.action === 'recent') {
    return {
      provider: 'plex',
      action: 'recent',
      results: LIBRARY
    };
  }

  if (input.action === 'search') {
    const terms = (input.query ?? '').toLowerCase().split(/\s+/u).filter((term) => term.length > 1);
    const results = LIBRARY.filter((item) => {
      const haystack = `${item.title} ${item.type}`.toLowerCase();
      return terms.some((term) => haystack.includes(term));
    });

    return {
      provider: 'plex',
      action: 'search',
      results
    };
  }

  return {
    provider: 'plex',
    action: 'play',
    now_playing: LIBRARY.find((item) => item.id === input.media_id) ?? LIBRARY[0]
  };
}

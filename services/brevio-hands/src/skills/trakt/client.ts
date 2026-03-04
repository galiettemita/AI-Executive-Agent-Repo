import type { TraktInput, TraktItem, TraktOutput } from './types.js';

const ITEMS: TraktItem[] = [
  {
    id: 'trakt_movie_001',
    title: 'Signal Paths',
    media_type: 'movie',
    year: 2025
  },
  {
    id: 'trakt_show_010',
    title: 'Inside Ops',
    media_type: 'show',
    year: 2026
  }
];

export async function runClient(input: TraktInput): Promise<TraktOutput> {
  if (input.action === 'history') {
    return {
      provider: 'trakt',
      action: 'history',
      items: ITEMS
    };
  }

  if (input.action === 'trending') {
    return {
      provider: 'trakt',
      action: 'trending',
      items: ITEMS
    };
  }

  return {
    provider: 'trakt',
    action: 'mark_watched',
    marked_watched: true
  };
}

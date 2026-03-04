import type { RadarrInput, RadarrMovie, RadarrOutput } from './types.js';

const BASE_QUEUE: RadarrMovie[] = [
  {
    movie_id: 'tmdb-603',
    title: 'The Matrix',
    status: 'queued',
    quality_profile: 'HD-1080p'
  },
  {
    movie_id: 'tmdb-27205',
    title: 'Inception',
    status: 'monitored',
    quality_profile: '4K-UHD'
  }
];

export async function runClient(input: RadarrInput): Promise<RadarrOutput> {
  if (input.action === 'list_queue') {
    return {
      provider: 'radarr',
      action: input.action,
      movies: BASE_QUEUE,
      queue_count: BASE_QUEUE.length,
      summary: `Radarr queue currently tracks ${BASE_QUEUE.length} movies.`
    };
  }

  if (input.action === 'search_movie') {
    return {
      provider: 'radarr',
      action: input.action,
      movies: [
        {
          movie_id: 'tmdb-search',
          title: input.query ?? 'Unknown',
          status: 'queued',
          quality_profile: input.quality_profile ?? 'HD-1080p'
        }
      ],
      queue_count: 1,
      summary: `Found 1 movie candidate for "${input.query}".`
    };
  }

  return {
    provider: 'radarr',
    action: input.action,
    movies: [
      {
        movie_id: input.tmdb_id ?? 'tmdb-unknown',
        title: 'Added movie',
        status: 'monitored',
        quality_profile: input.quality_profile ?? 'HD-1080p'
      }
    ],
    queue_count: 1,
    summary: `Added movie ${input.tmdb_id} to Radarr monitoring.`
  };
}

import type { OverseerrInput, OverseerrOutput, OverseerrRequest } from './types.js';

const BASE_REQUESTS: OverseerrRequest[] = [
  {
    request_id: 'ovr-101',
    media_id: 'tmdb-603',
    title: 'The Matrix',
    media_type: 'movie',
    status: 'approved'
  },
  {
    request_id: 'ovr-102',
    media_id: 'tvdb-121361',
    title: 'Westworld',
    media_type: 'tv',
    status: 'pending'
  }
];

export async function runClient(input: OverseerrInput): Promise<OverseerrOutput> {
  if (input.action === 'list_requests') {
    return {
      provider: 'overseerr',
      action: input.action,
      requests: BASE_REQUESTS,
      summary: `Loaded ${BASE_REQUESTS.length} Overseerr requests.`
    };
  }

  if (input.action === 'search_media') {
    const result: OverseerrRequest = {
      request_id: 'ovr-search',
      media_id: 'tmdb-999',
      title: input.query ?? 'Unknown title',
      media_type: input.media_type ?? 'movie',
      status: 'pending'
    };

    return {
      provider: 'overseerr',
      action: input.action,
      requests: [result],
      summary: `Found 1 result for query "${input.query}".`
    };
  }

  return {
    provider: 'overseerr',
    action: input.action,
    requests: [
      {
        request_id: 'ovr-new',
        media_id: input.media_id ?? 'unknown',
        title: 'Requested title',
        media_type: input.media_type ?? 'movie',
        status: 'pending'
      }
    ],
    summary: `Queued media request for ${input.media_id}.`
  };
}

import type {
  StreamingRecommendation,
  StreamingRecommendationsInput,
  StreamingRecommendationsOutput
} from './types.js';

const PICKS: StreamingRecommendation[] = [
  {
    title: 'The Executive Signal',
    type: 'series',
    genre: 'Drama',
    reason: 'Strong strategic decision-making arc.',
    available_on: ['Netflix', 'Max']
  },
  {
    title: 'Systems at Dawn',
    type: 'movie',
    genre: 'Documentary',
    reason: 'High-signal operations and leadership themes.',
    available_on: ['Prime Video']
  }
];

export async function runClient(
  input: StreamingRecommendationsInput
): Promise<StreamingRecommendationsOutput> {
  if (input.action === 'watchlist_add') {
    return {
      provider: 'streaming-recommendations',
      action: 'watchlist_add',
      watchlist_added: true,
      partnership_status: 'awaiting_api_partnership'
    };
  }

  if (input.action === 'watchlist_list') {
    return {
      provider: 'streaming-recommendations',
      action: 'watchlist_list',
      recommendations: PICKS,
      partnership_status: 'awaiting_api_partnership'
    };
  }

  return {
    provider: 'streaming-recommendations',
    action: 'recommend',
    recommendations: PICKS,
    partnership_status: 'awaiting_api_partnership'
  };
}

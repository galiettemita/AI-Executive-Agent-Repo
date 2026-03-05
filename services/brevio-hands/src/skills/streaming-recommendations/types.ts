export interface StreamingRecommendationsInput {
  action: 'recommend' | 'watchlist_add' | 'watchlist_list';
  mood?: string;
  genre?: string;
  title?: string;
  confirmed?: boolean;
}

export interface StreamingRecommendation {
  title: string;
  type: 'movie' | 'series';
  genre: string;
  reason: string;
  available_on: string[];
}

export interface StreamingRecommendationsOutput {
  provider: 'streaming-recommendations';
  action: StreamingRecommendationsInput['action'];
  recommendations?: StreamingRecommendation[];
  watchlist_added?: boolean;
  partnership_status: 'awaiting_api_partnership';
}

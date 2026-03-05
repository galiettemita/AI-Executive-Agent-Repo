export type SpotifyHistoryAction = 'top_tracks' | 'top_artists' | 'listening_summary';

export interface SpotifyHistoryInput {
  action: SpotifyHistoryAction;
  window?: '4w' | '6m' | '12m';
  limit?: number;
}

export interface SpotifyHistoryOutput {
  provider: 'spotify-history';
  action: SpotifyHistoryAction;
  top_tracks: Array<{ title: string; artist: string; play_count: number }>;
  top_artists: Array<{ name: string; play_count: number }>;
  total_listening_minutes: number;
  summary: string;
}

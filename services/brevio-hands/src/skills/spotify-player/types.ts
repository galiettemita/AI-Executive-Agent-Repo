export type SpotifyPlayerAction = 'search_tracks' | 'queue_track' | 'playback_status';

export interface SpotifyPlayerInput {
  action: SpotifyPlayerAction;
  query?: string;
  track_id?: string;
}

export interface SpotifyPlayerOutput {
  provider: 'spotify-player';
  action: SpotifyPlayerAction;
  tracks: Array<{ track_id: string; title: string; artist: string; duration_seconds: number }>;
  queue_length: number;
  summary: string;
}

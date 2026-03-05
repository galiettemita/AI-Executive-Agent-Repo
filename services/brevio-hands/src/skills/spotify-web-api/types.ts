export type SpotifyWebAction = 'playback' | 'search' | 'history' | 'top_tracks';

export interface SpotifyWebInput {
  action: SpotifyWebAction;
  query?: string;
}

export interface SpotifyTrack {
  track_id: string;
  name: string;
  artist: string;
}

export interface SpotifyWebOutput {
  provider: 'spotify-web-api';
  action: SpotifyWebAction;
  playing?: {
    track_id: string;
    name: string;
    artist: string;
    progress_ms: number;
  };
  results?: SpotifyTrack[];
}

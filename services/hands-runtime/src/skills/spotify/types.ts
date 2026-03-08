export type SpotifyAction = 'play' | 'pause' | 'next' | 'previous' | 'set_volume' | 'status';

export interface SpotifyInput {
  action: SpotifyAction;
  query?: string;
  volume_pct?: number;
}

export interface SpotifyOutput {
  provider: 'spotify';
  action: SpotifyAction;
  now_playing: {
    track: string;
    artist: string;
    is_playing: boolean;
    volume_pct: number;
    device: string;
  };
  summary: string;
}

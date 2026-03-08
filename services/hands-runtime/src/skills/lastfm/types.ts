export interface LastfmInput {
  action: 'recent_tracks' | 'top_tracks' | 'artist_summary';
  username?: string;
  artist?: string;
}

export interface LastfmTrack {
  name: string;
  artist: string;
  playcount: number;
}

export interface LastfmOutput {
  provider: 'lastfm';
  action: LastfmInput['action'];
  tracks?: LastfmTrack[];
  artist_summary?: {
    artist: string;
    listeners: number;
    summary: string;
  };
}

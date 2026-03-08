export interface AppleMusicInput {
  action: 'search' | 'play' | 'add_to_playlist';
  query?: string;
  playlist_id?: string;
  track_id?: string;
}

export interface AppleMusicTrack {
  id: string;
  title: string;
  artist: string;
  album: string;
}

export interface AppleMusicOutput {
  provider: 'apple-music';
  action: AppleMusicInput['action'];
  tracks?: AppleMusicTrack[];
  now_playing?: AppleMusicTrack;
  playlist_updated?: boolean;
}

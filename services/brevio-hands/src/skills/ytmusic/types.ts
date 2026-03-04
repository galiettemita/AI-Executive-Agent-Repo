export interface YTMusicInput {
  action: 'search' | 'play' | 'queue';
  query?: string;
  track_id?: string;
}

export interface YTMusicTrack {
  id: string;
  title: string;
  artist: string;
}

export interface YTMusicOutput {
  provider: 'ytmusic';
  action: YTMusicInput['action'];
  tracks?: YTMusicTrack[];
  now_playing?: YTMusicTrack;
  queued?: boolean;
}

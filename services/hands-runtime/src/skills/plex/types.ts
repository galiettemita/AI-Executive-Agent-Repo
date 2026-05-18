export interface PlexInput {
  action: 'search' | 'play' | 'recent';
  query?: string;
  media_id?: string;
}

export interface PlexMediaItem {
  id: string;
  title: string;
  type: 'movie' | 'episode' | 'album';
  year?: number;
}

export interface PlexOutput {
  provider: 'plex';
  action: PlexInput['action'];
  results?: PlexMediaItem[];
  now_playing?: PlexMediaItem;
}

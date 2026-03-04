export type YouTubeMode = 'search' | 'transcript' | 'channel';

export interface YouTubeSearchResult {
  video_id: string;
  title: string;
  channel: string;
  published_at: string;
}

export interface YouTubeInput {
  mode: YouTubeMode;
  query?: string;
  video_id?: string;
  channel_id?: string;
}

export interface YouTubeOutput {
  provider: 'youtube';
  mode: YouTubeMode;
  results?: YouTubeSearchResult[];
  transcript?: string;
}

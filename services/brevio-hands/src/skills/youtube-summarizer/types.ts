export type YoutubeSummarizerAction = 'summarize_video' | 'key_points';

export interface YoutubeSummarizerInput {
  action: YoutubeSummarizerAction;
  video_id?: string;
  video_url?: string;
  max_points?: number;
}

export interface YoutubeSummarizerOutput {
  provider: 'youtube-summarizer';
  action: YoutubeSummarizerAction;
  video_id: string;
  summary: string;
  key_points: string[];
  transcript_excerpt: string;
}

export type VideoTranscriptDownloaderAction = 'fetch_transcript' | 'fetch_subtitles';

export interface VideoTranscriptDownloaderInput {
  action: VideoTranscriptDownloaderAction;
  video_id?: string;
  video_url?: string;
  language?: string;
}

export interface VideoTranscriptDownloaderOutput {
  provider: 'video-transcript-downloader';
  action: VideoTranscriptDownloaderAction;
  video_id: string;
  language: string;
  transcript_text: string;
  segment_count: number;
  subtitle_url?: string;
}

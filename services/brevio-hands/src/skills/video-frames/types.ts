export type VideoFramesAction = 'extract_frame' | 'extract_frames';

export interface VideoFramesInput {
  action: VideoFramesAction;
  video_url: string;
  timestamp_seconds?: number;
  frame_interval_seconds?: number;
  frame_count?: number;
}

export interface VideoFramesOutput {
  provider: 'video-frames';
  action: VideoFramesAction;
  frame_urls: string[];
  extracted_count: number;
  summary: string;
}

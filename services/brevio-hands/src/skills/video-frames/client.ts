import type { VideoFramesInput, VideoFramesOutput } from './types.js';

function baseUrl(videoUrl: string): string {
  return `https://cdn.brevio.local/frames/${encodeURIComponent(videoUrl).slice(0, 30)}`;
}

export async function runClient(input: VideoFramesInput): Promise<VideoFramesOutput> {
  if (input.action === 'extract_frame') {
    const url = `${baseUrl(input.video_url)}-t${input.timestamp_seconds ?? 0}.jpg`;
    return {
      provider: 'video-frames',
      action: input.action,
      frame_urls: [url],
      extracted_count: 1,
      summary: `Extracted single frame at ${input.timestamp_seconds}s.`
    };
  }

  const count = input.frame_count ?? 1;
  const interval = input.frame_interval_seconds ?? 1;
  const frame_urls = Array.from({ length: count }, (_, index) => {
    return `${baseUrl(input.video_url)}-t${index * interval}.jpg`;
  });

  return {
    provider: 'video-frames',
    action: input.action,
    frame_urls,
    extracted_count: frame_urls.length,
    summary: `Extracted ${frame_urls.length} frame(s) at ${interval}s interval.`
  };
}

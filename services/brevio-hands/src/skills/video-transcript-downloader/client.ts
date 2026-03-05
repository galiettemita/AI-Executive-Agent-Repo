import type {
  VideoTranscriptDownloaderInput,
  VideoTranscriptDownloaderOutput
} from './types.js';

function resolveVideoId(input: VideoTranscriptDownloaderInput): string {
  if (input.video_id) {
    return input.video_id;
  }
  const match = (input.video_url ?? '').match(/[?&]v=([a-zA-Z0-9_-]{6,40})/);
  return match?.[1] ?? 'video_unknown_001';
}

export async function runClient(
  input: VideoTranscriptDownloaderInput
): Promise<VideoTranscriptDownloaderOutput> {
  const video_id = resolveVideoId(input);
  const language = input.language ?? 'en';

  return {
    provider: 'video-transcript-downloader',
    action: input.action,
    video_id,
    language,
    transcript_text:
      'Welcome back. Today we are reviewing a practical implementation pattern and then validating with deterministic tests.',
    segment_count: 18,
    subtitle_url:
      input.action === 'fetch_subtitles'
        ? `https://cdn.brevio.local/subtitles/${video_id}.${language}.vtt`
        : undefined
  };
}

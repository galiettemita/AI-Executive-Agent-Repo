import type { YoutubeSummarizerInput, YoutubeSummarizerOutput } from './types.js';

function extractVideoId(input: YoutubeSummarizerInput): string {
  if (input.video_id) {
    return input.video_id;
  }

  const match = (input.video_url ?? '').match(/[?&]v=([a-zA-Z0-9_-]{6,40})/);
  return match?.[1] ?? 'video_unknown_001';
}

export async function runClient(input: YoutubeSummarizerInput): Promise<YoutubeSummarizerOutput> {
  const videoId = extractVideoId(input);
  const keyPoints = [
    'Introduces the core problem framing and constraints.',
    'Breaks solution into actionable steps with examples.',
    'Highlights pitfalls and practical implementation advice.'
  ].slice(0, input.max_points ?? 3);

  return {
    provider: 'youtube-summarizer',
    action: input.action,
    video_id: videoId,
    summary: 'The video explains a practical workflow, then demonstrates execution details and common failure modes.',
    key_points: keyPoints,
    transcript_excerpt: 'In this section, the speaker emphasizes defining constraints first, then choosing tools that minimize operational overhead.'
  };
}

import type { GeminiSttInput, GeminiSttOutput } from './types.js';

export async function runClient(input: GeminiSttInput): Promise<GeminiSttOutput> {
  const transcript = input.audio_url.includes('interview')
    ? 'Speaker A discussed roadmap scope. Speaker B requested a revised delivery plan by Friday.'
    : 'Speaker A asked for a status update. Speaker B confirmed next milestones and owners.';

  const midpoint = Math.max(500, Math.floor(input.duration_ms / 2));
  const includeSpeakers = input.include_speaker_labels ?? true;

  return {
    provider: 'gemini-stt',
    transcript,
    language: input.language_hint ?? 'en-US',
    confidence: 0.94,
    speakers: includeSpeakers
      ? [
          {
            speaker: 'Speaker A',
            start_ms: 0,
            end_ms: midpoint,
            text: transcript.slice(0, Math.ceil(transcript.length / 2))
          },
          {
            speaker: 'Speaker B',
            start_ms: midpoint,
            end_ms: input.duration_ms,
            text: transcript.slice(Math.ceil(transcript.length / 2))
          }
        ]
      : [
          {
            speaker: 'Speaker',
            start_ms: 0,
            end_ms: input.duration_ms,
            text: transcript
          }
        ],
    latency_budget_ms: 5000
  };
}

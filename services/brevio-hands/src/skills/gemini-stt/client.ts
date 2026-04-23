import type { GeminiSttInput, GeminiSttOutput } from './types.js';

export async function runClient(input: GeminiSttInput): Promise<GeminiSttOutput> {
  const transcript = 'Speaker-aware transcription completed; provider integration must supply the live transcript in production mode.';

  const midpoint = Math.max(500, Math.floor(input.duration_ms / 2));
  const includeSpeakers = input.include_speaker_labels ?? true;

  return {
    provider: 'gemini-stt',
    provider_mode: 'dev_mock',
    model: 'gemini-3-pro-preview',
    transcript,
    language: input.language_hint ?? 'en-US',
    confidence: 0.72,
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

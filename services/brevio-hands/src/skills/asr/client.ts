import type { AsrInput, AsrOutput } from './types.js';

export async function runClient(input: AsrInput): Promise<AsrOutput> {
  const transcript = 'Audio transcription completed; provider integration must supply the live transcript in production mode.';
  const midpoint = Math.max(500, Math.floor(input.duration_ms / 2));
  return {
    provider: 'asr',
    provider_mode: 'dev_mock',
    model: 'gpt-4o-transcribe',
    transcript,
    language: input.language_hint ?? 'en-US',
    confidence: 0.7,
    segments: [
      {
        start_ms: 0,
        end_ms: midpoint,
        text: transcript.slice(0, Math.ceil(transcript.length / 2))
      },
      {
        start_ms: midpoint,
        end_ms: input.duration_ms,
        text: transcript.slice(Math.ceil(transcript.length / 2))
      }
    ],
    word_timestamps: [],
    latency_budget_ms: 3000
  };
}

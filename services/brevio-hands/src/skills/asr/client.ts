import type { AsrInput, AsrOutput } from './types.js';

function deriveTranscript(input: AsrInput): string {
  if (input.audio_url.includes('meeting')) {
    return 'Please summarize my 10am leadership meeting and capture action items.';
  }

  if (input.audio_url.includes('groceries')) {
    return 'Add eggs, avocados, and oat milk to my grocery list for this week.';
  }

  return 'Draft a concise update for the team and schedule a follow up tomorrow morning.';
}

export async function runClient(input: AsrInput): Promise<AsrOutput> {
  const transcript = deriveTranscript(input);
  const midpoint = Math.max(500, Math.floor(input.duration_ms / 2));
  return {
    provider: 'asr',
    transcript,
    language: input.language_hint ?? 'en-US',
    confidence: 0.91,
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
    latency_budget_ms: 3000
  };
}

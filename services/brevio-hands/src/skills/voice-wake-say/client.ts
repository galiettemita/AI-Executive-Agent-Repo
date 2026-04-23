import type { VoiceWakeSayInput, VoiceWakeSayOutput } from './types.js';

export async function runClient(input: VoiceWakeSayInput): Promise<VoiceWakeSayOutput> {
  const voice = input.voice ?? 'Alex';
  const rate = input.rate_wpm ?? 180;

  return {
    provider: 'voice-wake-say',
    voice,
    command_argv: ['say', '-v', voice, '-r', String(rate), '--', input.text],
    estimated_duration_ms: Math.max(400, input.text.length * 28),
    latency_budget_ms: 500
  };
}

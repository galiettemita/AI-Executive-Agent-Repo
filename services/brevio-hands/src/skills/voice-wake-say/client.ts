import type { VoiceWakeSayInput, VoiceWakeSayOutput } from './types.js';

function escapeSayText(text: string): string {
  return text.replace(/"/g, "'");
}

export async function runClient(input: VoiceWakeSayInput): Promise<VoiceWakeSayOutput> {
  const voice = input.voice ?? 'Alex';
  const rate = input.rate_wpm ?? 180;
  const safeText = escapeSayText(input.text);

  return {
    provider: 'voice-wake-say',
    voice,
    command: `say -v ${voice} -r ${rate} "${safeText}"`,
    estimated_duration_ms: Math.max(400, input.text.length * 28),
    latency_budget_ms: 500
  };
}

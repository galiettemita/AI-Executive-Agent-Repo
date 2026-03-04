import type { SagInput, SagOutput } from './types.js';

function slugify(text: string): string {
  const slug = text
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, '-')
    .replace(/^-+|-+$/g, '')
    .slice(0, 40);
  return slug || 'speech';
}

export async function runClient(input: SagInput): Promise<SagOutput> {
  const voice_id = input.voice_id ?? 'elevenlabs-default-voice';
  const style = input.style ?? 'warm';

  return {
    provider: 'sag',
    voice_id,
    style,
    audio_url: `https://cdn.brevio.local/tts/sag/${slugify(input.text)}.mp3`,
    estimated_duration_ms: Math.max(1000, input.text.length * 36),
    latency_budget_ms: 3000
  };
}

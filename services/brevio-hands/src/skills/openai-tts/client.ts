import type { OpenAiTtsInput, OpenAiTtsOutput } from './types.js';

function slugify(text: string): string {
  const slug = text
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, '-')
    .replace(/^-+|-+$/g, '')
    .slice(0, 40);
  return slug || 'speech';
}

export async function runClient(input: OpenAiTtsInput): Promise<OpenAiTtsOutput> {
  const voice = input.voice ?? 'alloy';
  const format = input.format ?? 'mp3';
  return {
    provider: 'openai-tts',
    voice,
    format,
    audio_url: `https://cdn.brevio.local/tts/openai/${slugify(input.text)}.${format}`,
    estimated_duration_ms: Math.max(900, input.text.length * 34),
    latency_budget_ms: 2000
  };
}

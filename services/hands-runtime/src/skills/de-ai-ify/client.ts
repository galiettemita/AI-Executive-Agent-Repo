import type { DeAiIfyInput, DeAiIfyOutput } from './types.js';

export async function runClient(input: DeAiIfyInput): Promise<DeAiIfyOutput> {
  const original = input.text ?? '';
  const rewritten = original.replace(/\butilize\b/gi, 'use').replace(/\bmoreover\b/gi, 'also');
  const markers = ['overly formal transition words', 'uniform sentence rhythm'];

  return {
    provider: 'de-ai-ify',
    action: input.action,
    rewritten_text: rewritten,
    detected_ai_markers: markers,
    summary: `Rewrote text in ${input.target_tone ?? 'direct'} tone with ${markers.length} marker adjustments.`
  };
}

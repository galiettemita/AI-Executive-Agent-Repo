import type { KreaApiInput, KreaApiOutput } from './types.js';

export async function runClient(input: KreaApiInput): Promise<KreaApiOutput> {
  const model = input.model ?? 'krea-flux';

  return {
    provider: 'krea-api',
    action: input.action,
    image_url: input.action === 'upscale_image' ? input.image_url ?? 'https://assets.brevio.local/krea/upscaled.png' : 'https://assets.brevio.local/krea/generated.png',
    model,
    quality_score: input.action === 'upscale_image' ? 0.94 : 0.88,
    summary: `Krea action ${input.action} completed using model ${model}.`
  };
}

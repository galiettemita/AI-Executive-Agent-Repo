import type { PollinationsInput, PollinationsOutput } from './types.js';

export async function runClient(input: PollinationsInput): Promise<PollinationsOutput> {
  const model = input.model ?? 'pollinations-diffusion';
  const extension = input.action === 'generate_video' ? 'mp4' : input.action === 'generate_audio' ? 'wav' : 'png';

  return {
    provider: 'pollinations',
    action: input.action,
    asset_url: `https://assets.brevio.local/pollinations/output.${extension}`,
    model_used: model,
    summary: `Pollinations generated ${input.action.replace('generate_', '')} with model ${model}.`
  };
}

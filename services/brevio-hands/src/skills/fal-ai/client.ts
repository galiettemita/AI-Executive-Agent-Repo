import { createHash } from 'node:crypto';

import type { FalAIInput, FalAIOutput } from './types.js';

const BLOCKED_TERMS = ['exploit', 'self-harm', 'csam'];

export async function runClient(input: FalAIInput): Promise<FalAIOutput> {
  const lower = input.prompt.toLowerCase();
  for (const term of BLOCKED_TERMS) {
    if (lower.includes(term)) {
      throw new Error('FAL_CONTENT_POLICY_BLOCKED');
    }
  }

  const model = input.model ?? 'fal-ai/flux/dev';
  const size = input.size ?? 'square';
  const digest = createHash('sha256').update(`${model}|${size}|${input.prompt}`).digest('hex').slice(0, 20);

  return {
    provider: 'fal-ai',
    image_url: `https://cdn.mock.fal.ai/${digest}.png`,
    model_used: model,
    size
  };
}

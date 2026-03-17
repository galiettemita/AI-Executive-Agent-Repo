// Plan §6 step 9 — Real fal.ai image generation API
// Authorization: Key <key>

interface SkillContext { token?: string; user_id?: string; }

const SIZE_MAP: Record<string, { width: number; height: number }> = {
  square:    { width: 1024, height: 1024 },
  landscape: { width: 1280, height:  720 },
  portrait:  { width:  720, height: 1280 },
};

export async function runClient(
  input: Record<string, any>,
  ctx?: SkillContext
): Promise<any> {
  const key = process.env.FAL_KEY;
  if (!key)           throw new Error('FAL_KEY env var is required');
  if (!input.prompt)  throw new Error('prompt is required');

  const model      = input.model ?? 'fal-ai/fast-sdxl';
  const image_size = SIZE_MAP[input.size ?? 'square'] ?? SIZE_MAP['square'];

  const res = await fetch(`https://fal.run/${model}`, {
    method:  'POST',
    headers: {
      authorization:  `Key ${key}`,
      'content-type': 'application/json',
    },
    body: JSON.stringify({
      prompt:     input.prompt,
      image_size,
      num_images: input.num_images ?? 1,
    }),
  });
  if (!res.ok) {
    const errBody = await res.json().catch(() => ({}));
    throw new Error(`fal.ai API error ${res.status}: ${JSON.stringify(errBody)}`);
  }
  const body = await res.json();

  return {
    image_url: body.images[0].url,
    images:    body.images.map((img: any) => img.url),
    model,
    prompt:    input.prompt,
    seed:      body.seed ?? null,
    size:      input.size ?? 'square',
  };
}

import type { ColoringPageInput, ColoringPageOutput } from './types.js';

export async function runClient(input: ColoringPageInput): Promise<ColoringPageOutput> {
  const complexity = input.complexity ?? 'medium';
  const density = complexity === 'easy' ? 'low' : complexity === 'advanced' ? 'high' : 'medium';

  return {
    provider: 'coloring-page',
    action: input.action,
    output_url: 'https://assets.brevio.local/coloring-page-output.pdf',
    page_size: 'Letter',
    line_density: density,
    summary: `Generated coloring page from ${input.action === 'generate_from_prompt' ? 'prompt' : 'image'} with ${density} line density.`
  };
}

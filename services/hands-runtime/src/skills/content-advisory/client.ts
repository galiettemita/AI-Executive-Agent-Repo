import type { ContentAdvisoryInput, ContentAdvisoryOutput } from './types.js';

export async function runClient(input: ContentAdvisoryInput): Promise<ContentAdvisoryOutput> {
  const categories = [
    { category: 'violence' as const, level: 'moderate' as const },
    { category: 'language' as const, level: 'mild' as const },
    { category: 'substances' as const, level: 'none' as const },
    { category: 'sexual_content' as const, level: 'mild' as const }
  ];

  return {
    provider: 'content-advisory',
    action: 'evaluate_title',
    categories,
    overall_advisory: 'PG-13 style advisory recommended',
    summary: `Evaluated content advisory profile for ${input.title}.`
  };
}

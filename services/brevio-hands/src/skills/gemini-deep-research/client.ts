import type { GeminiDeepResearchInput, GeminiDeepResearchOutput } from './types.js';

export async function runClient(input: GeminiDeepResearchInput): Promise<GeminiDeepResearchOutput> {
  return {
    provider: 'gemini-deep-research',
    action: 'run_research',
    report_sections: ['Executive summary', 'Key findings', 'Risks and follow-ups'],
    citations: ['https://research.example.com/citation-1'],
    summary: `Generated ${input.depth ?? 'standard'} deep-research brief for ${input.topic}.`
  };
}

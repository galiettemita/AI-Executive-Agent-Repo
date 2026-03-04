import type { ResumeBuilderInput, ResumeBuilderOutput } from './types.js';

const BASE_RESUME = `# Alex Executive\n\n## Summary\nOperations-focused leader with a track record of delivery and automation.\n\n## Experience\n- Scaled cross-functional ops cadence across 4 teams.\n- Reduced response times through deterministic workflow routing.`;

export async function runClient(input: ResumeBuilderInput): Promise<ResumeBuilderOutput> {
  if (input.action === 'generate') {
    return {
      provider: 'resume-builder',
      action: 'generate',
      resume_markdown: `${BASE_RESUME}\n- Target Role: ${input.role ?? 'Generalist'}`,
      recommendations: ['Add quantified outcomes for each bullet.', 'Include leadership scope.']
    };
  }

  if (input.action === 'tailor') {
    return {
      provider: 'resume-builder',
      action: 'tailor',
      resume_markdown: `${BASE_RESUME}\n\n## Tailoring Notes\nAligned to ${input.role ?? 'target role'} with JD keywords inserted.`,
      recommendations: ['Match top three skills from the job description in summary.']
    };
  }

  return {
    provider: 'resume-builder',
    action: 'score',
    score: 87,
    recommendations: ['Add a dedicated achievements section.', 'Shorten the summary by one sentence.']
  };
}

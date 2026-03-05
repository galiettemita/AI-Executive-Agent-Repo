import type { ThinkingPartnerInput, ThinkingPartnerMatrixRow, ThinkingPartnerOutput } from './types.js';

function buildQuestions(topic: string, constraints: string[]): string[] {
  return [
    `What outcome would make this a clear win: ${topic}?`,
    'Which assumption, if false, would invalidate your current plan?',
    `What can be decided today despite constraints: ${constraints.join(', ') || 'none provided'}?`
  ];
}

function buildAssumptions(constraints: string[]): string[] {
  if (!constraints.length) {
    return [
      'Current timeline assumptions are realistic.',
      'Resource availability will remain stable.',
      'Stakeholders share the same success criteria.'
    ];
  }

  return constraints.map((constraint) => `Constraint remains binding: ${constraint}`);
}

function buildDecisionMatrix(options: string[]): ThinkingPartnerMatrixRow[] {
  return options.map((option, index) => ({
    option,
    expected_upside: `Primary upside for ${option} is faster progress toward objective.`,
    key_risk: `Primary risk for ${option} is execution complexity under uncertainty.`,
    confidence_score: Math.max(3, 9 - index)
  }));
}

export async function runClient(input: ThinkingPartnerInput): Promise<ThinkingPartnerOutput> {
  const topic = input.topic ?? 'Unspecified decision';
  const constraints = input.constraints ?? [];
  const questions = buildQuestions(topic, constraints);
  const assumptions_to_test = buildAssumptions(constraints);

  if (input.action === 'decision_matrix') {
    const matrix = buildDecisionMatrix(input.options ?? []);
    return {
      provider: 'thinking-partner',
      action: input.action,
      reframed_problem: `Choose the best option for: ${topic}`,
      questions,
      assumptions_to_test,
      decision_matrix: matrix
    };
  }

  return {
    provider: 'thinking-partner',
    action: input.action,
    reframed_problem: `Clarify the highest-leverage move for: ${topic}`,
    questions,
    assumptions_to_test
  };
}

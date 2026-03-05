import type { RelationshipSkillsInput, RelationshipSkillsOutput } from './types.js';

export async function runClient(input: RelationshipSkillsInput): Promise<RelationshipSkillsOutput> {
  const points = [
    'Start with a concrete observation without blaming language.',
    'State your need clearly and ask one open-ended question.'
  ];

  return {
    provider: 'relationship-skills',
    action: input.action,
    talking_points: points,
    suggested_message: `I want to share what I noticed and work together on ${input.goal}.`,
    summary: `Prepared ${points.length} communication points for ${input.action}.`
  };
}

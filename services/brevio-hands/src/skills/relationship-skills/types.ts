export type RelationshipSkillsAction = 'coach_message' | 'conflict_plan';

export interface RelationshipSkillsInput {
  action: RelationshipSkillsAction;
  context?: string;
  goal?: string;
}

export interface RelationshipSkillsOutput {
  provider: 'relationship-skills';
  action: RelationshipSkillsAction;
  talking_points: string[];
  suggested_message: string;
  summary: string;
}

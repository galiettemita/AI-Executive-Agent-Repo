export interface SkillInputPayload {
  payload?: Record<string, unknown>;
}

export interface SkillOutputPayload {
  ok: boolean;
  skill_id: string;
}

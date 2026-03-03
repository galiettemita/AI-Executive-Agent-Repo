import type { SkillInputPayload, SkillOutputPayload } from './types.js';

export async function runClient(input: SkillInputPayload): Promise<SkillOutputPayload> {
  void input;
  return { ok: true, skill_id: 'lastfm' };
}

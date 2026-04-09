function optionalString(value: unknown): string | undefined {
  if (typeof value !== 'string') {
    return undefined;
  }
  const trimmed = value.trim();
  return trimmed.length > 0 ? trimmed : undefined;
}

export interface LocalSkillExecutionSuccess {
  data: Record<string, unknown>;
}

type LocalSkillHandler = (input: Record<string, unknown>) => LocalSkillExecutionSuccess;

const LOCAL_SKILL_HANDLERS: Record<string, LocalSkillHandler> = {
  'voice-wake-say': (input) => ({
    data: {
      spoken_text: optionalString(input.text) ?? 'done',
      transport: 'local_say'
    }
  }),
  'apple-remind-me': (input) => ({
    data: {
      reminder_title: optionalString(input.title) ?? 'Reminder from Brevio',
      created: true
    }
  })
};

export function implementedLocalSkills(): string[] {
  return Object.keys(LOCAL_SKILL_HANDLERS);
}

export function resolveSupportedLocalSkills(raw: string | undefined): string[] {
  const implemented = new Set(implementedLocalSkills());
  const configured = raw
    ? raw
        .split(',')
        .map((skill) => skill.trim().toLowerCase())
        .filter((skill) => skill !== '')
    : implementedLocalSkills();

  const seen = new Set<string>();
  const supported: string[] = [];
  for (const skill of configured) {
    if (implemented.has(skill) && !seen.has(skill)) {
      supported.push(skill);
      seen.add(skill);
    }
  }
  return supported;
}

export function executeImplementedLocalSkill(skillId: string, input: Record<string, unknown>): LocalSkillExecutionSuccess | null {
  const handler = LOCAL_SKILL_HANDLERS[skillId.trim().toLowerCase()];
  return handler ? handler(input) : null;
}

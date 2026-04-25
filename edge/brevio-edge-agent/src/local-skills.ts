function optionalString(value: unknown): string | undefined {
  if (typeof value !== 'string') {
    return undefined;
  }
  const trimmed = value.trim();
  return trimmed.length > 0 ? trimmed : undefined;
}

export interface LocalSkillExecution {
  status: 'SUCCESS' | 'NEEDS_CONSENT' | 'NOT_EXECUTED' | 'SIMULATED';
  data?: Record<string, unknown>;
  error?: {
    code: string;
    message: string;
  };
}

interface LocalSkillDefinition {
  operations: string[];
  execute: (input: Record<string, unknown>, operation: string) => LocalSkillExecution;
}

const LOCAL_SKILL_DEFINITIONS: Record<string, LocalSkillDefinition> = {
  'voice-wake-say': {
    operations: ['speak'],
    execute: (input) => ({
      status: 'SUCCESS',
      data: {
        spoken_text: optionalString(input.text) ?? 'done',
        transport: 'local_say',
        command_argv: ['say', '--', optionalString(input.text) ?? 'done']
      }
    })
  },
  'camsnap': {
    operations: ['capture', 'describe'],
    execute: (_input, operation) => ({
      status: 'NEEDS_CONSENT',
      data: {
        provider: 'camsnap',
        operation,
        status: 'permission_required',
        consent_required: true,
        output_modalities: ['image']
      }
    })
  },
  'apple-photos': {
    operations: ['search', 'describe'],
    execute: (input, operation) => ({
      status: 'NEEDS_CONSENT',
      data: {
        provider: 'apple-photos',
        operation,
        query: optionalString(input.query) ?? '',
        status: 'permission_required',
        consent_required: true,
        output_modalities: ['image', 'document']
      }
    })
  },
  'apple-media': {
    operations: ['search', 'open'],
    execute: (input, operation) => ({
      status: 'NEEDS_CONSENT',
      data: {
        provider: 'apple-media',
        operation,
        query: optionalString(input.query) ?? '',
        status: 'permission_required',
        consent_required: true,
        output_modalities: ['audio', 'video', 'document']
      }
    })
  },
  'apple-remind-me': {
    operations: ['create'],
    execute: (input) => ({
      status: 'SIMULATED',
      data: {
        reminder_title: optionalString(input.title) ?? 'Reminder from Brevio',
        created: false,
        simulated: true
      }
    })
  }
};

export function implementedLocalSkills(): string[] {
  return Object.keys(LOCAL_SKILL_DEFINITIONS);
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

export function supportsLocalOperation(skillId: string, operation: string): boolean {
  const definition = LOCAL_SKILL_DEFINITIONS[skillId.trim().toLowerCase()];
  return Boolean(definition?.operations.includes(operation.trim().toLowerCase()));
}

export function executeImplementedLocalSkill(
  skillId: string,
  operation: string,
  input: Record<string, unknown>
): LocalSkillExecution | null {
  const definition = LOCAL_SKILL_DEFINITIONS[skillId.trim().toLowerCase()];
  if (!definition || !definition.operations.includes(operation.trim().toLowerCase())) {
    return null;
  }
  return definition.execute(input, operation.trim().toLowerCase());
}

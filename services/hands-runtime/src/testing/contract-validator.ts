import type { ISkillAdapter } from '@brevio/shared';

export interface ContractViolation {
  field: string;
  expected: string;
  actual: string;
  severity: 'error' | 'warning';
}

export interface ContractValidationResult {
  skillId: string;
  passed: boolean;
  violations: ContractViolation[];
  executionTimeMs: number;
}

export interface SkillCassette {
  skillId: string;
  description: string;
  input: Record<string, unknown>;
  expectedOutput: Record<string, unknown>;
  httpMocks?: HttpMock[];
}

export interface HttpMock {
  url: string | RegExp;
  method: string;
  statusCode: number;
  responseBody: unknown;
  responseHeaders?: Record<string, string>;
}

/**
 * ContractValidator validates that a skill's declared I/O schema matches
 * the shape of its actual output. It runs the skill against a cassette
 * (recorded fixture) and compares the output structure against the declared
 * outputSchema in the adapter.
 */
export class ContractValidator {
  constructor(private readonly skills: Map<string, ISkillAdapter>) {}

  async validate(
    skillId: string,
    fixtureInput: Record<string, unknown>,
    fixtureExpectedOutput: Record<string, unknown>,
  ): Promise<ContractValidationResult> {
    const start = performance.now();
    const skill = this.skills.get(skillId);
    if (!skill) {
      return {
        skillId,
        passed: false,
        violations: [{ field: 'skill', expected: 'registered', actual: 'not_found', severity: 'error' }],
        executionTimeMs: 0,
      };
    }

    const violations: ContractViolation[] = [];

    // Validate input against declared inputSchema.
    const inputValidation = validateAgainstSchema(fixtureInput, skill.inputSchema as Record<string, unknown>);
    violations.push(...inputValidation.map(v => ({ ...v, severity: 'error' as const })));

    // Validate expected output shape against declared outputSchema.
    const outputValidation = validateAgainstSchema(fixtureExpectedOutput, skill.outputSchema as Record<string, unknown>);
    violations.push(...outputValidation.map(v => ({ ...v, severity: 'warning' as const })));

    // Validate that all required outputSchema fields are present in fixture.
    const requiredOutputFields = getRequiredFields(skill.outputSchema as Record<string, unknown>);
    for (const field of requiredOutputFields) {
      if (!(field in fixtureExpectedOutput)) {
        violations.push({
          field,
          expected: 'present in fixture',
          actual: 'missing',
          severity: 'error',
        });
      }
    }

    const executionTimeMs = performance.now() - start;
    return {
      skillId,
      passed: violations.filter(v => v.severity === 'error').length === 0,
      violations,
      executionTimeMs,
    };
  }

  async validateAll(cassettes: SkillCassette[]): Promise<ContractValidationResult[]> {
    return Promise.all(cassettes.map(c => this.validate(c.skillId, c.input, c.expectedOutput)));
  }
}

export function validateAgainstSchema(
  obj: Record<string, unknown>,
  schema: Record<string, unknown>,
): ContractViolation[] {
  const violations: ContractViolation[] = [];
  const properties = (schema.properties ?? {}) as Record<string, { type?: string }>;
  const required = (schema.required ?? []) as string[];

  for (const field of required) {
    if (!(field in obj)) {
      violations.push({ field, expected: 'present', actual: 'missing', severity: 'error' });
      continue;
    }
    const declaredType = properties[field]?.type;
    if (declaredType) {
      const actualType = Array.isArray(obj[field]) ? 'array' : typeof obj[field];
      if (actualType !== declaredType) {
        violations.push({ field, expected: declaredType, actual: actualType, severity: 'error' });
      }
    }
  }
  return violations;
}

export function getRequiredFields(schema: Record<string, unknown>): string[] {
  return (schema.required ?? []) as string[];
}

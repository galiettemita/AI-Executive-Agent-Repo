import assert from 'node:assert/strict';
import { describe, it } from 'node:test';
import { ContractValidator, validateAgainstSchema, type SkillCassette } from '../contract-validator.js';
import type { ISkillAdapter, SkillInput, SkillContext, SkillResult } from '@brevio/shared';

function mockSkill(id: string, inputRequired: string[], outputRequired: string[]): ISkillAdapter {
  return {
    id,
    plane: 'hands',
    requiredScopes: [],
    inputSchema: {
      type: 'object',
      required: inputRequired,
      properties: Object.fromEntries(inputRequired.map(f => [f, { type: 'string' }])),
    },
    outputSchema: {
      type: 'object',
      required: outputRequired,
      properties: Object.fromEntries(outputRequired.map(f => [f, { type: 'string' }])),
    },
    async execute(_input: SkillInput, _ctx: SkillContext): Promise<SkillResult> {
      return { skill_id: id, status: 'SUCCESS', latency_ms: 0, metadata: {} };
    },
    async healthCheck() { return true; },
  };
}

describe('ContractValidator', () => {
  it('1. passes on valid cassette', async () => {
    const skill = mockSkill('test-skill', ['query'], ['provider', 'results']);
    const validator = new ContractValidator(new Map([['test-skill', skill]]));
    const result = await validator.validate(
      'test-skill',
      { query: 'hello' },
      { provider: 'test', results: 'some-results' },
    );
    assert.ok(result.passed);
    assert.equal(result.violations.filter(v => v.severity === 'error').length, 0);
  });

  it('2. fails on missing required output field', async () => {
    const skill = mockSkill('test-skill', ['query'], ['provider', 'results']);
    const validator = new ContractValidator(new Map([['test-skill', skill]]));
    const result = await validator.validate(
      'test-skill',
      { query: 'hello' },
      { provider: 'test' }, // missing 'results'
    );
    assert.ok(!result.passed);
    const missing = result.violations.find(v => v.field === 'results' && v.severity === 'error');
    assert.ok(missing, 'should flag missing required output field');
  });

  it('3. fails on type violation', async () => {
    const skill: ISkillAdapter = {
      ...mockSkill('test-skill', [], ['events']),
      outputSchema: {
        type: 'object',
        required: ['events'],
        properties: { events: { type: 'array' } },
      },
    };
    const validator = new ContractValidator(new Map([['test-skill', skill]]));
    const result = await validator.validate(
      'test-skill',
      {},
      { events: 'not-an-array' }, // string instead of array
    );
    const typeViolation = result.violations.find(v => v.field === 'events');
    assert.ok(typeViolation, 'should flag type mismatch');
    assert.equal(typeViolation?.expected, 'array');
    assert.equal(typeViolation?.actual, 'string');
  });

  it('4. fails when skill not found', async () => {
    const validator = new ContractValidator(new Map());
    const result = await validator.validate('nonexistent', {}, {});
    assert.ok(!result.passed);
    assert.equal(result.violations[0]?.actual, 'not_found');
  });

  it('5. validateAgainstSchema detects missing required fields', () => {
    const schema = {
      type: 'object',
      required: ['name', 'age'],
      properties: { name: { type: 'string' }, age: { type: 'number' } },
    };
    const violations = validateAgainstSchema({ name: 'Alice' }, schema);
    assert.equal(violations.length, 1);
    assert.equal(violations[0]?.field, 'age');
  });

  it('6. validateAll returns batch results', async () => {
    const skill = mockSkill('batch-skill', ['q'], ['r']);
    const validator = new ContractValidator(new Map([['batch-skill', skill]]));
    const cassettes: SkillCassette[] = [
      { skillId: 'batch-skill', description: 'a', input: { q: '1' }, expectedOutput: { r: 'x' } },
      { skillId: 'batch-skill', description: 'b', input: { q: '2' }, expectedOutput: { r: 'y' } },
      { skillId: 'batch-skill', description: 'c', input: { q: '3' }, expectedOutput: { r: 'z' } },
    ];
    const results = await validator.validateAll(cassettes);
    assert.equal(results.length, 3);
    assert.ok(results.every(r => r.passed));
  });
});

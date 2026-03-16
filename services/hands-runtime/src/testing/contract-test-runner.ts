import { describe, it } from 'node:test';
import assert from 'node:assert/strict';
import { ContractValidator, type SkillCassette, type HttpMock } from './contract-validator.js';
import type { ISkillAdapter } from '@brevio/shared';

/**
 * runContractTests generates a node:test suite from a cassette array.
 * Call this from each skill's integration.test.ts.
 */
export function runContractTests(skill: ISkillAdapter, cassettes: SkillCassette[]): void {
  describe(`${skill.id} contract validation`, () => {
    for (const cassette of cassettes) {
      it(cassette.description, async () => {
        const validator = new ContractValidator(new Map([[skill.id, skill]]));

        // Mock fetch for HTTP cassettes.
        const originalFetch = globalThis.fetch;
        if (cassette.httpMocks && cassette.httpMocks.length > 0) {
          (globalThis as Record<string, unknown>)['fetch'] = createMockFetch(cassette.httpMocks);
        }

        try {
          const result = await validator.validate(skill.id, cassette.input, cassette.expectedOutput);
          const errors = result.violations.filter(v => v.severity === 'error');
          assert.equal(errors.length, 0,
            `Contract violations:\n${JSON.stringify(errors, null, 2)}`);
          assert.ok(result.passed);
        } finally {
          (globalThis as Record<string, unknown>)['fetch'] = originalFetch;
        }
      });
    }
  });
}

function createMockFetch(mocks: HttpMock[]) {
  return async (url: string | URL | Request, options?: RequestInit) => {
    const urlStr = typeof url === 'string' ? url : url instanceof URL ? url.toString() : url.url;
    const method = options?.method ?? 'GET';
    const match = mocks.find(m => {
      const urlMatch = typeof m.url === 'string' ? urlStr.includes(m.url) : m.url.test(urlStr);
      return urlMatch && m.method.toUpperCase() === method.toUpperCase();
    });
    if (!match) {
      throw new Error(`Unmocked HTTP call: ${method} ${urlStr}`);
    }
    return new Response(JSON.stringify(match.responseBody), {
      status: match.statusCode,
      headers: { 'Content-Type': 'application/json', ...(match.responseHeaders ?? {}) },
    });
  };
}

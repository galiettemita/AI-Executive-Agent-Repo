import type { ISkillAdapter, SkillContext, SkillInput, SkillResult } from '@brevio/shared';

import { runClient } from './client.js';
import { InputSchema, OutputSchema } from './schema.js';
import type { PdfToolsInput, PdfToolsOutput } from './types.js';

const PDF_TOOLS_MERGE_FILES_REQUIRED = 'PDF_TOOLS_MERGE_FILES_REQUIRED';

const adapter: ISkillAdapter = {
  id: 'pdf-tools',
  plane: 'hands',
  requiredScopes: [],
  inputSchema: {
    type: 'object',
    required: ['action', 'files'],
    properties: {
      action: { type: 'string', enum: ['extract_text', 'merge', 'split'] },
      files: { type: 'array' },
      page_range: { type: 'string', pattern: '^\\d+-\\d+$' },
      output_name: { type: 'string', minLength: 1, maxLength: 120 }
    },
    additionalProperties: false
  },
  outputSchema: {
    type: 'object',
    required: ['provider', 'action', 'output_path', 'pages_processed'],
    properties: {
      provider: { type: 'string', enum: ['pdf-tools'] },
      action: { type: 'string', enum: ['extract_text', 'merge', 'split'] },
      output_path: { type: 'string' },
      pages_processed: { type: 'integer' },
      extracted_text_preview: { type: 'string' }
    },
    additionalProperties: false
  },
  async execute(input: SkillInput, _ctx: SkillContext): Promise<SkillResult> {
    const started = Date.now();
    const parsed = InputSchema.safeParse(input);
    if (!parsed.success) {
      return {
        skill_id: 'pdf-tools',
        status: 'FAILED',
        error: {
          code: 'VALIDATION_FAILED',
          message:
            parsed.error.issues.map((issue) => issue.message).join('; ') || PDF_TOOLS_MERGE_FILES_REQUIRED,
          retryable: false,
          http_status: 400
        },
        latency_ms: Date.now() - started,
        metadata: {
          retries: 0,
          circuit_breaker_state: 'CLOSED',
          cache_hit: false
        }
      };
    }

    try {
      const data = await runClient(parsed.data as PdfToolsInput);
      const output = OutputSchema.parse(data) as PdfToolsOutput;
      return {
        skill_id: 'pdf-tools',
        status: 'SUCCESS',
        data: output,
        latency_ms: Date.now() - started,
        metadata: {
          retries: 0,
          circuit_breaker_state: 'CLOSED',
          cache_hit: false
        }
      };
    } catch (err) {
      return {
        skill_id: 'pdf-tools',
        status: 'FAILED',
        error: {
          code: 'EXTERNAL_ERROR',
          message: err instanceof Error ? err.message : 'pdf-tools execution failed',
          retryable: true,
          http_status: 502
        },
        latency_ms: Date.now() - started,
        metadata: {
          retries: 0,
          circuit_breaker_state: 'CLOSED',
          cache_hit: false
        }
      };
    }
  },
  async healthCheck(): Promise<boolean> {
    return true;
  }
};

export default adapter;

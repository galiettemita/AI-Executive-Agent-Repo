import type { ISkillAdapter, SkillContext, SkillInput, SkillResult } from '@brevio/shared';

import { runClient } from './client.js';
import { InputSchema, OutputSchema } from './schema.js';
import type { ExcalidrawFlowchartInput, ExcalidrawFlowchartOutput } from './types.js';

const EXCALIDRAW_DESCRIPTION_REQUIRED = 'EXCALIDRAW_DESCRIPTION_REQUIRED';

const adapter: ISkillAdapter = {
  id: 'excalidraw-flowchart',
  plane: 'hands',
  requiredScopes: ['diagram.write'],
  inputSchema: {
    type: 'object',
    required: ['action'],
    properties: {
      action: { type: 'string', enum: ['generate_flowchart', 'update_flowchart', 'export_svg'] },
      description: { type: 'string', minLength: 2, maxLength: 1000 },
      flowchart_id: { type: 'string', minLength: 3, maxLength: 120 },
      nodes: { type: 'array', items: { type: 'string' }, minItems: 1, maxItems: 50 }
    },
    additionalProperties: false
  },
  outputSchema: {
    type: 'object',
    required: ['provider', 'action', 'flowchart_id', 'nodes', 'edges', 'summary'],
    properties: {
      provider: { type: 'string', enum: ['excalidraw-flowchart'] },
      action: { type: 'string', enum: ['generate_flowchart', 'update_flowchart', 'export_svg'] },
      flowchart_id: { type: 'string' },
      nodes: { type: 'array', items: { type: 'object' } },
      edges: { type: 'array', items: { type: 'object' } },
      export_url: { type: 'string' },
      summary: { type: 'string' }
    },
    additionalProperties: false
  },
  async execute(input: SkillInput, _ctx: SkillContext): Promise<SkillResult> {
    const started = Date.now();
    const parsed = InputSchema.safeParse(input);
    if (!parsed.success) {
      return {
        skill_id: 'excalidraw-flowchart',
        status: 'FAILED',
        error: {
          code: 'VALIDATION_FAILED',
          message:
            parsed.error.issues.map((issue) => issue.message).join('; ') || EXCALIDRAW_DESCRIPTION_REQUIRED,
          retryable: false,
          http_status: 400
        },
        latency_ms: Date.now() - started,
        metadata: { retries: 0, circuit_breaker_state: 'CLOSED', cache_hit: false }
      };
    }

    try {
      const data = await runClient(parsed.data as ExcalidrawFlowchartInput);
      const output = OutputSchema.parse(data) as ExcalidrawFlowchartOutput;
      return {
        skill_id: 'excalidraw-flowchart',
        status: 'SUCCESS',
        data: output,
        latency_ms: Date.now() - started,
        metadata: { retries: 0, circuit_breaker_state: 'CLOSED', cache_hit: false }
      };
    } catch (err) {
      return {
        skill_id: 'excalidraw-flowchart',
        status: 'FAILED',
        error: {
          code: 'EXTERNAL_ERROR',
          message: err instanceof Error ? err.message : 'excalidraw-flowchart execution failed',
          retryable: true,
          http_status: 502
        },
        latency_ms: Date.now() - started,
        metadata: { retries: 0, circuit_breaker_state: 'CLOSED', cache_hit: false }
      };
    }
  },
  async healthCheck(): Promise<boolean> {
    return true;
  }
};

export default adapter;

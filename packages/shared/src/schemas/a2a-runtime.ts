import type { JSONSchema7 } from 'json-schema';
import { z } from 'zod';

export const AgentCapabilitySchema = z.object({
  id: z.string().min(1),
  name: z.string().min(1),
  description: z.string().min(1),
  version: z.string().min(1),
  input_modes: z.array(z.string()).min(1),
  output_modes: z.array(z.string()).min(1),
  async: z.boolean().default(true)
});

export const AgentCardSchema = z.object({
  agent_id: z.string().min(1),
  name: z.string().min(1),
  description: z.string().min(1),
  version: z.string().min(1),
  protocol_version: z.string().min(1),
  default_endpoint: z.string().min(1),
  capabilities: z.array(AgentCapabilitySchema).min(1),
  supports: z.object({
    task_lifecycle: z.boolean(),
    task_query: z.boolean(),
    artifact_updates: z.boolean(),
    push_callbacks: z.boolean(),
    capability_inventory: z.boolean()
  })
});

export const AgentExecutionRefSchema = z.object({
  request_id: z.string().min(1),
  run_id: z.string().min(1).optional(),
  task_id: z.string().min(1).optional(),
  parent_task_id: z.string().min(1).optional(),
  step_id: z.string().min(1).optional(),
  attempt: z.number().int().positive().optional()
});

export const AgentTaskArtifactSchema = z.object({
  artifact_id: z.string().min(1),
  type: z.string().min(1),
  uri: z.string().min(1).optional(),
  inline_data: z.unknown().optional()
});

export const AgentTaskStateSchema = z.enum([
  'QUEUED',
  'WAITING_FOR_AGENT',
  'DISPATCHED',
  'SUCCESS',
  'PARTIAL',
  'FAILED',
  'TIMEOUT',
  'REJECTED'
]);

export const AgentTaskSchema = AgentExecutionRefSchema.extend({
  agent_id: z.string().min(1),
  skill_id: z.string().min(1),
  user_id: z.string().min(1),
  device_id: z.string().min(1).optional(),
  status: AgentTaskStateSchema,
  created_at: z.string().datetime(),
  updated_at: z.string().datetime(),
  queued_at: z.string().datetime().optional(),
  dispatched_at: z.string().datetime().optional(),
  completed_at: z.string().datetime().optional(),
  artifacts: z.array(AgentTaskArtifactSchema).optional(),
  last_error: z
    .object({
      code: z.string().min(1),
      message: z.string().min(1)
    })
    .optional()
});

export const CapabilityInventoryEntrySchema = z.object({
  scope: z.enum(['tenant', 'user', 'device']),
  scope_id: z.string().min(1),
  skill_id: z.string().min(1),
  enabled: z.boolean(),
  updated_at: z.string().datetime()
});

export const AgentCardJsonSchema: JSONSchema7 = {
  $schema: 'http://json-schema.org/draft-07/schema#',
  $id: 'https://schemas.brevio.app/agent-card.v1.json',
  type: 'object',
  additionalProperties: false,
  required: ['agent_id', 'name', 'description', 'version', 'protocol_version', 'default_endpoint', 'capabilities', 'supports'],
  properties: {
    agent_id: { type: 'string', minLength: 1 },
    name: { type: 'string', minLength: 1 },
    description: { type: 'string', minLength: 1 },
    version: { type: 'string', minLength: 1 },
    protocol_version: { type: 'string', minLength: 1 },
    default_endpoint: { type: 'string', minLength: 1 },
    capabilities: {
      type: 'array',
      minItems: 1,
      items: {
        type: 'object',
        additionalProperties: false,
        required: ['id', 'name', 'description', 'version', 'input_modes', 'output_modes', 'async'],
        properties: {
          id: { type: 'string', minLength: 1 },
          name: { type: 'string', minLength: 1 },
          description: { type: 'string', minLength: 1 },
          version: { type: 'string', minLength: 1 },
          input_modes: { type: 'array', minItems: 1, items: { type: 'string' } },
          output_modes: { type: 'array', minItems: 1, items: { type: 'string' } },
          async: { type: 'boolean' }
        }
      }
    },
    supports: {
      type: 'object',
      additionalProperties: false,
      required: ['task_lifecycle', 'task_query', 'artifact_updates', 'push_callbacks', 'capability_inventory'],
      properties: {
        task_lifecycle: { type: 'boolean' },
        task_query: { type: 'boolean' },
        artifact_updates: { type: 'boolean' },
        push_callbacks: { type: 'boolean' },
        capability_inventory: { type: 'boolean' }
      }
    }
  }
};

export const AgentTaskJsonSchema: JSONSchema7 = {
  $schema: 'http://json-schema.org/draft-07/schema#',
  $id: 'https://schemas.brevio.app/agent-task.v1.json',
  type: 'object',
  additionalProperties: false,
  required: ['request_id', 'agent_id', 'skill_id', 'user_id', 'status', 'created_at', 'updated_at'],
  properties: {
    request_id: { type: 'string', minLength: 1 },
    run_id: { type: 'string', minLength: 1 },
    task_id: { type: 'string', minLength: 1 },
    parent_task_id: { type: 'string', minLength: 1 },
    step_id: { type: 'string', minLength: 1 },
    attempt: { type: 'integer', minimum: 1 },
    agent_id: { type: 'string', minLength: 1 },
    skill_id: { type: 'string', minLength: 1 },
    user_id: { type: 'string', minLength: 1 },
    device_id: { type: 'string', minLength: 1 },
    status: {
      type: 'string',
      enum: ['QUEUED', 'WAITING_FOR_AGENT', 'DISPATCHED', 'SUCCESS', 'PARTIAL', 'FAILED', 'TIMEOUT', 'REJECTED']
    },
    created_at: { type: 'string', format: 'date-time' },
    updated_at: { type: 'string', format: 'date-time' },
    queued_at: { type: 'string', format: 'date-time' },
    dispatched_at: { type: 'string', format: 'date-time' },
    completed_at: { type: 'string', format: 'date-time' },
    artifacts: {
      type: 'array',
      items: {
        type: 'object',
        additionalProperties: false,
        required: ['artifact_id', 'type'],
        properties: {
          artifact_id: { type: 'string', minLength: 1 },
          type: { type: 'string', minLength: 1 },
          uri: { type: 'string', minLength: 1 },
          inline_data: {}
        }
      }
    },
    last_error: {
      type: 'object',
      additionalProperties: false,
      required: ['code', 'message'],
      properties: {
        code: { type: 'string', minLength: 1 },
        message: { type: 'string', minLength: 1 }
      }
    }
  }
};

export type AgentCapability = z.infer<typeof AgentCapabilitySchema>;
export type AgentCard = z.infer<typeof AgentCardSchema>;
export type AgentExecutionRef = z.infer<typeof AgentExecutionRefSchema>;
export type AgentTaskArtifact = z.infer<typeof AgentTaskArtifactSchema>;
export type AgentTaskState = z.infer<typeof AgentTaskStateSchema>;
export type AgentTask = z.infer<typeof AgentTaskSchema>;
export type CapabilityInventoryEntry = z.infer<typeof CapabilityInventoryEntrySchema>;

import type { JSONSchema7 } from 'json-schema';
import { z } from 'zod';

export const MessageEnvelopeSchema = z.object({
  id: z.string().uuid(),
  channel: z.enum(['WHATSAPP', 'IMESSAGE', 'API']),
  user_id: z.string().uuid(),
  timestamp: z.string().datetime(),
  content: z.object({
    type: z.enum(['TEXT', 'VOICE', 'IMAGE', 'DOCUMENT', 'LOCATION']),
    text: z.string().max(4096).optional(),
    media_url: z.string().url().optional(),
    voice_duration_ms: z.number().int().max(120000).optional()
  }),
  metadata: z.object({
    channel_message_id: z.string(),
    reply_to: z.string().uuid().optional(),
    session_id: z.string().uuid()
  }),
  context: z.object({
    user_profile_hash: z.string().length(64),
    active_skills: z.array(z.string()).max(50).optional()
  }),
  routing: z
    .object({
      intent: z.string().optional(),
      skill_ids: z.array(z.string()).optional(),
      task_graph: z.unknown().optional()
    })
    .optional()
});

export const MessageEnvelopeJsonSchema: JSONSchema7 = {
  $schema: 'http://json-schema.org/draft-07/schema#',
  $id: 'https://schemas.brevio.app/message-envelope.v1.json',
  type: 'object',
  additionalProperties: false,
  required: ['id', 'channel', 'user_id', 'timestamp', 'content', 'metadata', 'context'],
  properties: {
    id: { type: 'string', format: 'uuid' },
    channel: { type: 'string', enum: ['WHATSAPP', 'IMESSAGE', 'API'] },
    user_id: { type: 'string', format: 'uuid' },
    timestamp: { type: 'string', format: 'date-time' },
    content: {
      type: 'object',
      additionalProperties: false,
      required: ['type'],
      properties: {
        type: { type: 'string', enum: ['TEXT', 'VOICE', 'IMAGE', 'DOCUMENT', 'LOCATION'] },
        text: { type: 'string', maxLength: 4096 },
        media_url: { type: 'string', format: 'uri' },
        voice_duration_ms: { type: 'integer', maximum: 120000 }
      }
    },
    metadata: {
      type: 'object',
      additionalProperties: false,
      required: ['channel_message_id', 'session_id'],
      properties: {
        channel_message_id: { type: 'string' },
        reply_to: { type: 'string', format: 'uuid' },
        session_id: { type: 'string', format: 'uuid' }
      }
    },
    context: {
      type: 'object',
      additionalProperties: false,
      required: ['user_profile_hash'],
      properties: {
        user_profile_hash: { type: 'string', minLength: 64, maxLength: 64 },
        active_skills: {
          type: 'array',
          maxItems: 50,
          items: { type: 'string' }
        }
      }
    },
    routing: {
      type: 'object',
      additionalProperties: false,
      properties: {
        intent: { type: 'string' },
        skill_ids: {
          type: 'array',
          items: { type: 'string' }
        },
        task_graph: {}
      }
    }
  }
};

export type MessageEnvelope = z.infer<typeof MessageEnvelopeSchema>;

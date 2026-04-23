import type { JSONSchema7 } from 'json-schema';
import { z } from 'zod';

const MediaAssetSchema = z.object({
  asset_id: z.string().min(1),
  mime_type: z.string().min(1),
  size_bytes: z.number().int().min(0).optional(),
  sha256: z.string().length(64).optional(),
  storage_uri: z.string().min(1).optional(),
  source_uri: z.string().url().optional(),
  filename: z.string().min(1).optional(),
  duration_ms: z.number().int().min(0).optional(),
  width: z.number().int().positive().optional(),
  height: z.number().int().positive().optional(),
  page_count: z.number().int().positive().optional(),
  codec: z.string().min(1).optional(),
  provenance: z.string().min(1).optional(),
  safety_labels: z.array(z.string()).optional(),
  metadata: z.record(z.unknown()).optional()
});

const ContentPartSchema = z.object({
  type: z.enum(['text', 'image', 'audio', 'video', 'document', 'location', 'tool_result', 'generated_asset', 'file']),
  text: z.string().max(4096).optional(),
  asset_id: z.string().min(1).optional(),
  media: MediaAssetSchema.optional()
});

export const MessageEnvelopeSchema = z.object({
  id: z.string().uuid(),
  channel: z.enum(['WHATSAPP', 'IMESSAGE', 'API']),
  user_id: z.string().uuid(),
  timestamp: z.string().datetime(),
  content: z.object({
    type: z.enum(['TEXT', 'VOICE', 'IMAGE', 'VIDEO', 'DOCUMENT', 'LOCATION', 'MULTIMODAL']),
    text: z.string().max(4096).optional(),
    media_url: z.string().url().optional(),
    voice_duration_ms: z.number().int().max(120000).optional(),
    parts: z.array(ContentPartSchema).max(50).optional(),
    media_assets: z.array(MediaAssetSchema).max(50).optional()
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
        type: { type: 'string', enum: ['TEXT', 'VOICE', 'IMAGE', 'VIDEO', 'DOCUMENT', 'LOCATION', 'MULTIMODAL'] },
        text: { type: 'string', maxLength: 4096 },
        media_url: { type: 'string', format: 'uri' },
        voice_duration_ms: { type: 'integer', maximum: 120000 },
        parts: {
          type: 'array',
          maxItems: 50,
          items: {
            type: 'object',
            additionalProperties: false,
            required: ['type'],
            properties: {
              type: {
                type: 'string',
                enum: ['text', 'image', 'audio', 'video', 'document', 'location', 'tool_result', 'generated_asset', 'file']
              },
              text: { type: 'string', maxLength: 4096 },
              asset_id: { type: 'string' },
              media: { $ref: '#/definitions/media_asset' }
            }
          }
        },
        media_assets: {
          type: 'array',
          maxItems: 50,
          items: { $ref: '#/definitions/media_asset' }
        }
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
  },
  definitions: {
    media_asset: {
      type: 'object',
      additionalProperties: false,
      required: ['asset_id', 'mime_type'],
      properties: {
        asset_id: { type: 'string', minLength: 1 },
        mime_type: { type: 'string', minLength: 1 },
        size_bytes: { type: 'integer', minimum: 0 },
        sha256: { type: 'string', minLength: 64, maxLength: 64 },
        storage_uri: { type: 'string' },
        source_uri: { type: 'string', format: 'uri' },
        filename: { type: 'string' },
        duration_ms: { type: 'integer', minimum: 0 },
        width: { type: 'integer', minimum: 1 },
        height: { type: 'integer', minimum: 1 },
        page_count: { type: 'integer', minimum: 1 },
        codec: { type: 'string' },
        provenance: { type: 'string' },
        safety_labels: { type: 'array', items: { type: 'string' } },
        metadata: { type: 'object' }
      }
    }
  }
};

export type MessageEnvelope = z.infer<typeof MessageEnvelopeSchema>;
export type MediaAsset = z.infer<typeof MediaAssetSchema>;
export type ContentPart = z.infer<typeof ContentPartSchema>;

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

export type MessageEnvelope = z.infer<typeof MessageEnvelopeSchema>;

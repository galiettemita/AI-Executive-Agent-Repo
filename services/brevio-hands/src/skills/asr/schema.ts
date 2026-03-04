import { z } from 'zod';

const SegmentSchema = z.object({
  start_ms: z.number().int().min(0),
  end_ms: z.number().int().positive(),
  text: z.string().min(1).max(500)
});

export const InputSchema = z
  .object({
    audio_url: z.string().url().startsWith('https://'),
    mime_type: z.enum(['audio/ogg', 'audio/mpeg', 'audio/wav', 'audio/mp4']),
    duration_ms: z.number().int().min(500).max(120000),
    language_hint: z.string().min(2).max(20).optional()
  })
  .strict();

export const OutputSchema = z
  .object({
    provider: z.literal('asr'),
    transcript: z.string().min(1).max(4096),
    language: z.string().min(2).max(20),
    confidence: z.number().min(0).max(1),
    segments: z.array(SegmentSchema).max(40),
    latency_budget_ms: z.literal(3000)
  })
  .strict();

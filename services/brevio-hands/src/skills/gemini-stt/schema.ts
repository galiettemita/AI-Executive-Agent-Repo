import { z } from 'zod';

const SpeakerSchema = z.object({
  speaker: z.string().min(2).max(40),
  start_ms: z.number().int().min(0),
  end_ms: z.number().int().positive(),
  text: z.string().min(1).max(500)
});

export const InputSchema = z
  .object({
    audio_url: z.string().url().startsWith('https://'),
    duration_ms: z.number().int().min(500).max(120000),
    language_hint: z.string().min(2).max(20).optional(),
    include_speaker_labels: z.boolean().optional()
  })
  .strict();

export const OutputSchema = z
  .object({
    provider: z.literal('gemini-stt'),
    provider_mode: z.enum(['dev_mock', 'live']),
    model: z.string().min(1),
    transcript: z.string().min(1).max(4096),
    language: z.string().min(2).max(20),
    confidence: z.number().min(0).max(1),
    speakers: z.array(SpeakerSchema).max(50),
    latency_budget_ms: z.literal(5000)
  })
  .strict();

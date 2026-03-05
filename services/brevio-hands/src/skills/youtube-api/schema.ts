import { z } from 'zod';

export const InputSchema = z
  .object({
    mode: z.enum(['search', 'transcript', 'channel']),
    query: z.string().min(2).max(300).optional(),
    video_id: z.string().min(3).max(80).optional(),
    channel_id: z.string().min(3).max(80).optional()
  })
  .strict();

const SearchResultSchema = z.object({
  video_id: z.string(),
  title: z.string(),
  channel: z.string(),
  published_at: z.string().datetime()
});

export const OutputSchema = z
  .object({
    provider: z.literal('youtube'),
    mode: z.enum(['search', 'transcript', 'channel']),
    results: z.array(SearchResultSchema).optional(),
    transcript: z.string().optional()
  })
  .strict();

import { z } from 'zod';

const ActionSchema = z.enum(['fetch_transcript', 'fetch_subtitles']);

export const InputSchema = z
  .object({
    action: ActionSchema,
    video_id: z.string().min(6).max(40).optional(),
    video_url: z.string().url().startsWith('https://').optional(),
    language: z.string().min(2).max(10).optional()
  })
  .strict()
  .superRefine((value, ctx) => {
    if (!value.video_id && !value.video_url) {
      ctx.addIssue({ code: z.ZodIssueCode.custom, message: 'VIDEO_TRANSCRIPT_VIDEO_REQUIRED' });
    }
  });

export const OutputSchema = z
  .object({
    provider: z.literal('video-transcript-downloader'),
    action: ActionSchema,
    video_id: z.string().min(6).max(40),
    language: z.string().min(2).max(10),
    transcript_text: z.string().min(10).max(12000),
    segment_count: z.number().int().min(1),
    subtitle_url: z.string().url().optional()
  })
  .strict();

import { z } from 'zod';

const ActionSchema = z.enum(['summarize_video', 'key_points']);

export const InputSchema = z
  .object({
    action: ActionSchema,
    video_id: z.string().min(6).max(40).optional(),
    video_url: z.string().url().startsWith('https://').optional(),
    max_points: z.number().int().min(1).max(20).optional()
  })
  .strict()
  .superRefine((value, ctx) => {
    if (!value.video_id && !value.video_url) {
      ctx.addIssue({ code: z.ZodIssueCode.custom, message: 'YOUTUBE_SUMMARIZER_VIDEO_REQUIRED' });
    }
  });

export const OutputSchema = z
  .object({
    provider: z.literal('youtube-summarizer'),
    action: ActionSchema,
    video_id: z.string().min(6).max(40),
    summary: z.string().min(10).max(4096),
    key_points: z.array(z.string().min(2).max(400)).max(20),
    transcript_excerpt: z.string().min(10).max(2000)
  })
  .strict();

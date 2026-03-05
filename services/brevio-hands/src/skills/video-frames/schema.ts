import { z } from 'zod';

const ActionSchema = z.enum(['extract_frame', 'extract_frames']);

export const InputSchema = z
  .object({
    action: ActionSchema,
    video_url: z.string().url().startsWith('https://'),
    timestamp_seconds: z.number().int().min(0).max(86400).optional(),
    frame_interval_seconds: z.number().int().min(1).max(3600).optional(),
    frame_count: z.number().int().min(1).max(200).optional()
  })
  .strict()
  .superRefine((value, ctx) => {
    if (value.action === 'extract_frame' && typeof value.timestamp_seconds !== 'number') {
      ctx.addIssue({ code: z.ZodIssueCode.custom, message: 'VIDEO_FRAMES_TIMESTAMP_REQUIRED' });
    }

    if (
      value.action === 'extract_frames' &&
      (typeof value.frame_interval_seconds !== 'number' || typeof value.frame_count !== 'number')
    ) {
      ctx.addIssue({ code: z.ZodIssueCode.custom, message: 'VIDEO_FRAMES_BATCH_FIELDS_REQUIRED' });
    }
  });

export const OutputSchema = z
  .object({
    provider: z.literal('video-frames'),
    action: ActionSchema,
    frame_urls: z.array(z.string().url()).max(200),
    extracted_count: z.number().int().min(1),
    summary: z.string().min(10).max(4096)
  })
  .strict();

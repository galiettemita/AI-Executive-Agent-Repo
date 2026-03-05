import { z } from 'zod';

const ActionSchema = z.enum(['list_channels', 'post_message', 'add_reaction']);

const ChannelSchema = z.object({
  id: z.string(),
  name: z.string()
});

const PostSchema = z.object({
  channel_id: z.string(),
  message_ts: z.string(),
  text: z.string()
});

export const InputSchema = z
  .object({
    action: ActionSchema,
    channel_id: z.string().min(3).max(50).optional(),
    text: z.string().min(1).max(4000).optional(),
    message_ts: z.string().min(3).max(50).optional(),
    emoji: z.string().min(2).max(40).optional()
  })
  .strict()
  .superRefine((value, ctx) => {
    if (value.action === 'post_message' && (!value.channel_id || !value.text)) {
      ctx.addIssue({
        code: z.ZodIssueCode.custom,
        message: 'SLACK_POST_FIELDS_REQUIRED'
      });
    }

    if (value.action === 'add_reaction' && (!value.channel_id || !value.message_ts || !value.emoji)) {
      ctx.addIssue({
        code: z.ZodIssueCode.custom,
        message: 'SLACK_REACTION_FIELDS_REQUIRED'
      });
    }
  });

export const OutputSchema = z
  .object({
    provider: z.literal('slack'),
    action: ActionSchema,
    channels: z.array(ChannelSchema).optional(),
    post: PostSchema.optional(),
    reacted: z.boolean().optional()
  })
  .strict();

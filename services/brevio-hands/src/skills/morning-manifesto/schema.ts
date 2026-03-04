import { z } from 'zod';

const ActionSchema = z.enum(['generate_manifesto', 'sync_actions']);

export const InputSchema = z
  .object({
    action: ActionSchema,
    goals: z.array(z.string().min(2).max(180)).max(20).optional(),
    gratitude: z.array(z.string().min(2).max(180)).max(10).optional(),
    blockers: z.array(z.string().min(2).max(180)).max(10).optional(),
    tone: z.enum(['direct', 'supportive']).optional(),
    sync_targets: z.array(z.enum(['apple_reminders', 'linear', 'obsidian'])).max(5).optional()
  })
  .strict()
  .superRefine((value, ctx) => {
    if (value.action === 'generate_manifesto' && !value.goals?.length) {
      ctx.addIssue({
        code: z.ZodIssueCode.custom,
        message: 'MORNING_MANIFESTO_GOALS_REQUIRED'
      });
    }

    if (value.action === 'sync_actions' && !value.sync_targets?.length) {
      ctx.addIssue({
        code: z.ZodIssueCode.custom,
        message: 'MORNING_MANIFESTO_SYNC_TARGETS_REQUIRED'
      });
    }
  });

export const OutputSchema = z
  .object({
    provider: z.literal('morning-manifesto'),
    action: ActionSchema,
    manifesto: z.string().min(10).max(4096),
    affirmations: z.array(z.string().min(2).max(180)).max(10),
    action_items: z.array(z.string().min(2).max(180)).max(20),
    sync_targets: z.array(z.string().min(2).max(80)).max(5)
  })
  .strict();

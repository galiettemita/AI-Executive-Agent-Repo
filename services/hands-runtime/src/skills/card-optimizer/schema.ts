import { z } from 'zod';

const ActionSchema = z.enum(['recommend_card', 'category_strategy']);

const CardSchema = z.object({
  card_name: z.string().min(2).max(160),
  reward_type: z.enum(['cashback', 'points', 'miles']),
  category_bonus_pct: z.number().min(0).max(100),
  base_pct: z.number().min(0).max(100)
});

export const InputSchema = z
  .object({
    action: ActionSchema,
    purchase_category: z.string().min(2).max(80).optional(),
    amount_cents: z.number().int().positive().max(100000000).optional(),
    available_cards: z.array(CardSchema).max(30).optional()
  })
  .strict()
  .superRefine((value, ctx) => {
    if (!value.purchase_category) {
      ctx.addIssue({ code: z.ZodIssueCode.custom, message: 'CARD_OPTIMIZER_CATEGORY_REQUIRED' });
    }
    if (!value.amount_cents) {
      ctx.addIssue({ code: z.ZodIssueCode.custom, message: 'CARD_OPTIMIZER_AMOUNT_REQUIRED' });
    }
  });

export const OutputSchema = z
  .object({
    provider: z.literal('card-optimizer'),
    action: ActionSchema,
    recommended_card: z.string().min(2).max(160),
    estimated_reward_cents: z.number().int().nonnegative(),
    alternatives: z
      .array(
        z.object({
          card_name: z.string().min(2).max(160),
          estimated_reward_cents: z.number().int().nonnegative()
        })
      )
      .max(30),
    rationale: z.string().min(10).max(4096)
  })
  .strict();

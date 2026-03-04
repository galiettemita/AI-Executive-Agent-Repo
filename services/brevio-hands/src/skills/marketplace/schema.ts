import { z } from 'zod';

const ActionSchema = z.enum(['evaluate_listing', 'compare_prices', 'draft_listing']);

export const InputSchema = z
  .object({
    action: ActionSchema,
    title: z.string().min(2).max(200).optional(),
    listing_url: z.string().url().startsWith('https://').optional(),
    condition: z.enum(['new', 'like_new', 'good', 'fair']).optional(),
    asking_price_cents: z.number().int().min(1).max(5000000).optional(),
    comparable_prices_cents: z.array(z.number().int().min(1).max(5000000)).max(100).optional()
  })
  .strict()
  .superRefine((value, ctx) => {
    if ((value.action === 'evaluate_listing' || value.action === 'draft_listing') && !value.title?.trim()) {
      ctx.addIssue({ code: z.ZodIssueCode.custom, message: 'MARKETPLACE_TITLE_REQUIRED' });
    }

    if (value.action === 'compare_prices' && !value.comparable_prices_cents?.length) {
      ctx.addIssue({ code: z.ZodIssueCode.custom, message: 'MARKETPLACE_COMPARABLES_REQUIRED' });
    }
  });

export const OutputSchema = z
  .object({
    provider: z.literal('marketplace'),
    action: ActionSchema,
    fair_price_cents: z.number().int().positive(),
    confidence: z.number().min(0).max(1),
    scam_risk: z.enum(['low', 'medium', 'high']),
    summary: z.string().min(10).max(4096),
    draft_listing_copy: z.string().min(10).max(4096).optional()
  })
  .strict();

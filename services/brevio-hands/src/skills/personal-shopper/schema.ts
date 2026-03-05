import { z } from 'zod';

const ActionSchema = z.enum(['research_product', 'rank_options', 'purchase_plan']);

const CandidateInputSchema = z.object({
  name: z.string().min(2).max(200),
  price_cents: z.number().int().min(1).max(10000000),
  features: z.array(z.string().min(2).max(120)).max(30).optional()
});

const CandidateOutputSchema = z.object({
  name: z.string().min(2).max(200),
  price_cents: z.number().int().positive(),
  score: z.number().int().min(1).max(100),
  pros: z.array(z.string().min(2).max(180)).max(10),
  cons: z.array(z.string().min(2).max(180)).max(10),
  buy_url: z.string().url().optional()
});

export const InputSchema = z
  .object({
    action: ActionSchema,
    query: z.string().min(2).max(200).optional(),
    budget_cents: z.number().int().min(1).max(10000000).optional(),
    constraints: z.array(z.string().min(2).max(180)).max(20).optional(),
    candidates: z.array(CandidateInputSchema).max(30).optional()
  })
  .strict()
  .superRefine((value, ctx) => {
    if (value.action === 'research_product' && !value.query?.trim()) {
      ctx.addIssue({ code: z.ZodIssueCode.custom, message: 'PERSONAL_SHOPPER_QUERY_REQUIRED' });
    }

    if ((value.action === 'rank_options' || value.action === 'purchase_plan') && !value.candidates?.length) {
      ctx.addIssue({ code: z.ZodIssueCode.custom, message: 'PERSONAL_SHOPPER_CANDIDATES_REQUIRED' });
    }
  });

export const OutputSchema = z
  .object({
    provider: z.literal('personal-shopper'),
    action: ActionSchema,
    summary: z.string().min(10).max(4096),
    ranked_candidates: z.array(CandidateOutputSchema).max(30),
    recommendation: z.string().min(5).max(4096),
    purchase_steps: z.array(z.string().min(2).max(240)).max(20).optional()
  })
  .strict();

import { z } from 'zod';

const ActionSchema = z.enum(['sentiment', 'volatility', 'correlation']);

const MetricSchema = z.object({
  symbol: z.string(),
  score: z.number(),
  summary: z.string()
});

const CorrelationSchema = z.object({
  symbols: z.array(z.string()),
  matrix: z.array(z.array(z.number()))
});

export const InputSchema = z
  .object({
    action: ActionSchema,
    symbols: z.array(z.string().regex(/^[A-Z.]{1,8}$/u)).min(1).max(10)
  })
  .strict()
  .superRefine((value, ctx) => {
    if (!value.symbols.length) {
      ctx.addIssue({
        code: z.ZodIssueCode.custom,
        message: 'FINANCIAL_MARKET_ANALYSIS_SYMBOLS_REQUIRED'
      });
    }
  });

export const OutputSchema = z
  .object({
    provider: z.literal('financial-market-analysis'),
    action: ActionSchema,
    metrics: z.array(MetricSchema).optional(),
    correlation: CorrelationSchema.optional()
  })
  .strict();

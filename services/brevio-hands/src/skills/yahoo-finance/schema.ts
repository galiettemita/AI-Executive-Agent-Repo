import { z } from 'zod';

const ActionSchema = z.enum(['quotes', 'fundamentals', 'news']);

const QuoteSchema = z.object({
  symbol: z.string(),
  price: z.number(),
  change_pct: z.number(),
  volume: z.number().int().nonnegative()
});

const FundamentalSchema = z.object({
  symbol: z.string(),
  market_cap: z.number().nonnegative(),
  pe_ratio: z.number().nonnegative().optional(),
  dividend_yield_pct: z.number().nonnegative().optional()
});

const NewsItemSchema = z.object({
  headline: z.string(),
  url: z.string().url(),
  published_at: z.string().datetime()
});

export const InputSchema = z
  .object({
    action: ActionSchema,
    symbols: z.array(z.string().regex(/^[A-Z.]{1,8}$/u)).min(1).max(20).optional()
  })
  .strict()
  .superRefine((value, ctx) => {
    if ((value.action === 'quotes' || value.action === 'fundamentals') && !value.symbols?.length) {
      ctx.addIssue({
        code: z.ZodIssueCode.custom,
        message: 'YAHOO_FINANCE_SYMBOLS_REQUIRED'
      });
    }
  });

export const OutputSchema = z
  .object({
    provider: z.literal('yahoo-finance'),
    action: ActionSchema,
    quotes: z.array(QuoteSchema).optional(),
    fundamentals: z.array(FundamentalSchema).optional(),
    news: z.array(NewsItemSchema).optional(),
    disclaimer: z.string()
  })
  .strict();

import { z } from 'zod';

const ActionSchema = z.enum(['quote_symbol', 'place_order', 'order_status']);

export const InputSchema = z
  .object({
    action: ActionSchema,
    symbol: z.string().min(1).max(12).optional(),
    side: z.enum(['BUY', 'SELL']).optional(),
    quantity: z.number().int().min(1).max(1000000).optional(),
    order_id: z.string().min(2).max(120).optional(),
    confirmed: z.boolean().optional()
  })
  .strict()
  .superRefine((value, ctx) => {
    if (value.action === 'quote_symbol' && !value.symbol) {
      ctx.addIssue({ code: z.ZodIssueCode.custom, message: 'IBKR_TRADING_SYMBOL_REQUIRED' });
    }
    if (value.action === 'place_order' && (!value.symbol || !value.side || typeof value.quantity !== 'number')) {
      ctx.addIssue({ code: z.ZodIssueCode.custom, message: 'IBKR_TRADING_ORDER_FIELDS_REQUIRED' });
    }
    if (value.action === 'place_order' && value.confirmed !== true) {
      ctx.addIssue({ code: z.ZodIssueCode.custom, message: 'IBKR_TRADING_CONFIRMATION_REQUIRED' });
    }
    if (value.action === 'order_status' && !value.order_id) {
      ctx.addIssue({ code: z.ZodIssueCode.custom, message: 'IBKR_TRADING_ORDER_ID_REQUIRED' });
    }
  });

export const OutputSchema = z
  .object({
    provider: z.literal('ibkr-trading'),
    action: ActionSchema,
    symbol: z.string().min(1).max(12),
    order_id: z.string().min(2).max(120).optional(),
    status: z.enum(['quoted', 'submitted', 'filled']),
    price_usd: z.number().min(0),
    summary: z.string().min(10).max(4096)
  })
  .strict();

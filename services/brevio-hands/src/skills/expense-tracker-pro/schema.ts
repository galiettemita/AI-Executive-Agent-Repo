import { z } from 'zod';

const ActionSchema = z.enum(['add_expense', 'monthly_summary', 'category_breakdown']);

const EntrySchema = z.object({
  entry_id: z.string().min(2).max(120),
  merchant: z.string().min(1).max(160),
  amount_cents: z.number().int().positive(),
  category: z.string().min(2).max(80),
  occurred_on: z.string().regex(/^\d{4}-\d{2}-\d{2}$/)
});

export const InputSchema = z
  .object({
    action: ActionSchema,
    merchant: z.string().min(1).max(160).optional(),
    amount_cents: z.number().int().positive().max(100000000).optional(),
    category: z.string().min(2).max(80).optional(),
    occurred_on: z.string().regex(/^\d{4}-\d{2}-\d{2}$/).optional(),
    month: z.string().regex(/^\d{4}-\d{2}$/).optional()
  })
  .strict()
  .superRefine((value, ctx) => {
    if (
      value.action === 'add_expense' &&
      (!value.merchant || !value.amount_cents || !value.category || !value.occurred_on)
    ) {
      ctx.addIssue({ code: z.ZodIssueCode.custom, message: 'EXPENSE_TRACKER_PRO_ADD_FIELDS_REQUIRED' });
    }
  });

export const OutputSchema = z
  .object({
    provider: z.literal('expense-tracker-pro'),
    action: ActionSchema,
    entries: z.array(EntrySchema).max(500),
    totals_by_category: z.record(z.number().int().nonnegative()),
    total_cents: z.number().int().nonnegative(),
    summary: z.string().min(10).max(4096)
  })
  .strict();

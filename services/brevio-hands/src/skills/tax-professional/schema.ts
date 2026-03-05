import { z } from 'zod';

const ActionSchema = z.enum(['estimate_deductions', 'filing_checklist']);

const ExpenseSchema = z.object({
  category: z.string().min(2).max(80),
  amount_cents: z.number().int().positive().max(1000000000)
});

export const InputSchema = z
  .object({
    action: ActionSchema,
    tax_year: z.number().int().min(2000).max(2100).optional(),
    filing_status: z
      .enum(['single', 'married_filing_jointly', 'married_filing_separately', 'head_of_household'])
      .optional(),
    deductible_expenses_cents: z.array(ExpenseSchema).max(500).optional()
  })
  .strict()
  .superRefine((value, ctx) => {
    if (!value.tax_year) {
      ctx.addIssue({ code: z.ZodIssueCode.custom, message: 'TAX_PROFESSIONAL_TAX_YEAR_REQUIRED' });
    }

    if (value.action === 'estimate_deductions' && !value.deductible_expenses_cents?.length) {
      ctx.addIssue({ code: z.ZodIssueCode.custom, message: 'TAX_PROFESSIONAL_EXPENSES_REQUIRED' });
    }
  });

export const OutputSchema = z
  .object({
    provider: z.literal('tax-professional'),
    action: ActionSchema,
    tax_year: z.number().int().min(2000).max(2100),
    estimated_deductions_cents: z.number().int().nonnegative(),
    checklist: z.array(z.string().min(2).max(240)).max(40),
    disclaimer: z.literal('not_tax_advice'),
    summary: z.string().min(10).max(4096)
  })
  .strict();

import type { TaxProfessionalInput, TaxProfessionalOutput } from './types.js';

const BASE_CHECKLIST = [
  'Gather W-2, 1099, and major income documentation.',
  'Collect deductible expense receipts and categorize them.',
  'Reconcile prior estimated payments and withholding amounts.',
  'Review filing status and dependent eligibility before submission.'
];

export async function runClient(input: TaxProfessionalInput): Promise<TaxProfessionalOutput> {
  const deductibleTotal = (input.deductible_expenses_cents ?? []).reduce(
    (sum, expense) => sum + expense.amount_cents,
    0
  );

  return {
    provider: 'tax-professional',
    action: input.action,
    tax_year: input.tax_year ?? 2026,
    estimated_deductions_cents: deductibleTotal,
    checklist: BASE_CHECKLIST,
    disclaimer: 'not_tax_advice',
    summary: `Prepared tax planning output for ${input.tax_year ?? 2026} with estimated deductions of $${(
      deductibleTotal / 100
    ).toFixed(2)}.`
  };
}

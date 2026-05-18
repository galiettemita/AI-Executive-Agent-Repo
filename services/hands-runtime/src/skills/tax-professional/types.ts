export type TaxProfessionalAction = 'estimate_deductions' | 'filing_checklist';

export interface TaxProfessionalInput {
  action: TaxProfessionalAction;
  tax_year?: number;
  filing_status?: 'single' | 'married_filing_jointly' | 'married_filing_separately' | 'head_of_household';
  deductible_expenses_cents?: Array<{
    category: string;
    amount_cents: number;
  }>;
}

export interface TaxProfessionalOutput {
  provider: 'tax-professional';
  action: TaxProfessionalAction;
  tax_year: number;
  estimated_deductions_cents: number;
  checklist: string[];
  disclaimer: 'not_tax_advice';
  summary: string;
}

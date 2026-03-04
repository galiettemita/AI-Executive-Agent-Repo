import type { ContractReviewerInput, ContractReviewerOutput } from './types.js';

export async function runClient(input: ContractReviewerInput): Promise<ContractReviewerOutput> {
  const riskItems = [
    {
      clause: 'Termination for convenience',
      severity: 'medium' as const,
      rationale: 'Notice period is short and asymmetric.'
    },
    {
      clause: 'Indemnification scope',
      severity: 'high' as const,
      rationale: 'Liability coverage appears one-sided.'
    }
  ];

  return {
    provider: 'contract-reviewer',
    action: input.action,
    overall_risk: 'medium',
    risk_items: riskItems,
    must_review_clauses: ['Liability cap', 'Termination rights', 'Data handling'],
    summary: `Reviewed ${input.contract_type ?? 'contract'} under ${input.jurisdiction ?? 'default'} assumptions.`
  };
}

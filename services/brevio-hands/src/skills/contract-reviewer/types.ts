export type ContractReviewerAction = 'review_contract' | 'summarize_risks';

export interface ContractReviewerInput {
  action: ContractReviewerAction;
  contract_text?: string;
  contract_type?: 'msa' | 'nda' | 'lease' | 'employment' | 'other';
  jurisdiction?: string;
}

export interface ContractRiskItem {
  clause: string;
  severity: 'low' | 'medium' | 'high';
  rationale: string;
}

export interface ContractReviewerOutput {
  provider: 'contract-reviewer';
  action: ContractReviewerAction;
  overall_risk: 'low' | 'medium' | 'high';
  risk_items: ContractRiskItem[];
  must_review_clauses: string[];
  summary: string;
}

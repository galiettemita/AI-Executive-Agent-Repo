export type ProsConsAction = 'evaluate_decision';

export interface ProsConsInput {
  action: ProsConsAction;
  decision?: string;
  options?: string[];
}

export interface DecisionOption {
  option: string;
  pros: string[];
  cons: string[];
  score: number;
}

export interface ProsConsOutput {
  provider: 'pros-cons';
  action: ProsConsAction;
  options: DecisionOption[];
  recommendation: string;
  summary: string;
}

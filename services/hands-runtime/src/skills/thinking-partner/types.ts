export type ThinkingPartnerAction = 'clarify_problem' | 'challenge_assumptions' | 'decision_matrix';

export interface ThinkingPartnerMatrixRow {
  option: string;
  expected_upside: string;
  key_risk: string;
  confidence_score: number;
}

export interface ThinkingPartnerInput {
  action: ThinkingPartnerAction;
  topic?: string;
  constraints?: string[];
  options?: string[];
}

export interface ThinkingPartnerOutput {
  provider: 'thinking-partner';
  action: ThinkingPartnerAction;
  reframed_problem: string;
  questions: string[];
  assumptions_to_test: string[];
  decision_matrix?: ThinkingPartnerMatrixRow[];
}

export type DeAiIfyAction = 'rewrite_text' | 'tone_check';

export interface DeAiIfyInput {
  action: DeAiIfyAction;
  text?: string;
  target_tone?: 'casual' | 'professional' | 'direct';
}

export interface DeAiIfyOutput {
  provider: 'de-ai-ify';
  action: DeAiIfyAction;
  rewritten_text: string;
  detected_ai_markers: string[];
  summary: string;
}

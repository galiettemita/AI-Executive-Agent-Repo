export interface SagInput {
  text: string;
  voice_id?: string;
  style?: 'neutral' | 'warm' | 'energetic';
}

export interface SagOutput {
  provider: 'sag';
  voice_id: string;
  style: 'neutral' | 'warm' | 'energetic';
  audio_url: string;
  estimated_duration_ms: number;
  latency_budget_ms: 3000;
}

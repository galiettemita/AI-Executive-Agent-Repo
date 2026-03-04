export interface VoiceWakeSayInput {
  text: string;
  voice?: string;
  rate_wpm?: number;
}

export interface VoiceWakeSayOutput {
  provider: 'voice-wake-say';
  voice: string;
  command: string;
  estimated_duration_ms: number;
  latency_budget_ms: 500;
}

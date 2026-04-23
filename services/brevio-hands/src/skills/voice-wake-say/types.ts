export interface VoiceWakeSayInput {
  text: string;
  voice?: 'Alex' | 'Samantha' | 'Victoria' | 'Daniel' | 'Moira';
  rate_wpm?: number;
}

export interface VoiceWakeSayOutput {
  provider: 'voice-wake-say';
  voice: 'Alex' | 'Samantha' | 'Victoria' | 'Daniel' | 'Moira';
  command_argv: string[];
  estimated_duration_ms: number;
  latency_budget_ms: 500;
}

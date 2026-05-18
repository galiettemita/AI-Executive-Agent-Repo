export interface OpenAiTtsInput {
  text: string;
  voice?: 'alloy' | 'verse' | 'sage';
  format?: 'mp3' | 'wav' | 'ogg';
}

export interface OpenAiTtsOutput {
  provider: 'openai-tts';
  voice: 'alloy' | 'verse' | 'sage';
  format: 'mp3' | 'wav' | 'ogg';
  audio_url: string;
  estimated_duration_ms: number;
  latency_budget_ms: 2000;
}

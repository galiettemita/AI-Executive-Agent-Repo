export type OpenAiTtsVoice = 'alloy' | 'ash' | 'ballad' | 'coral' | 'echo' | 'fable' | 'nova' | 'onyx' | 'sage' | 'shimmer' | 'verse';

export interface OpenAiTtsInput {
  text: string;
  voice?: OpenAiTtsVoice;
  format?: 'mp3' | 'wav' | 'ogg';
}

export interface OpenAiTtsOutput {
  provider: 'openai-tts';
  voice: OpenAiTtsVoice;
  format: 'mp3' | 'wav' | 'ogg';
  audio_url: string;
  estimated_duration_ms: number;
  latency_budget_ms: 2000;
}

export interface AsrSegment {
  start_ms: number;
  end_ms: number;
  text: string;
}

export interface AsrInput {
  audio_url: string;
  mime_type: 'audio/ogg' | 'audio/mpeg' | 'audio/wav' | 'audio/mp4';
  duration_ms: number;
  language_hint?: string;
}

export interface AsrOutput {
  provider: 'asr';
  provider_mode: 'dev_mock' | 'live';
  model: string;
  transcript: string;
  language: string;
  confidence: number;
  segments: AsrSegment[];
  word_timestamps: Array<{ word: string; start_ms: number; end_ms: number; confidence?: number }>;
  latency_budget_ms: 3000;
}

export interface GeminiSttSegment {
  speaker: string;
  start_ms: number;
  end_ms: number;
  text: string;
}

export interface GeminiSttInput {
  audio_url: string;
  duration_ms: number;
  language_hint?: string;
  include_speaker_labels?: boolean;
}

export interface GeminiSttOutput {
  provider: 'gemini-stt';
  transcript: string;
  language: string;
  confidence: number;
  speakers: GeminiSttSegment[];
  latency_budget_ms: 5000;
}

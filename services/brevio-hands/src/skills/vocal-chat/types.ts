export interface VocalChatInput {
  audio_url: string;
  mime_type: 'audio/ogg' | 'audio/mpeg' | 'audio/wav' | 'audio/mp4';
  duration_ms: number;
  response_voice?: 'alloy' | 'verse' | 'sage';
}

export interface VocalChatOutput {
  provider: 'vocal-chat';
  transcript: string;
  reply_text: string;
  reply_audio_url: string;
  stt_provider: 'asr' | 'gemini-stt';
  tts_provider: 'openai-tts';
  latency_budget_ms: 5000;
}

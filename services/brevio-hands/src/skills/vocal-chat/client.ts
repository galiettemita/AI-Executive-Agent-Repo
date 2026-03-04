import type { VocalChatInput, VocalChatOutput } from './types.js';

function deriveTranscript(input: VocalChatInput): string {
  if (input.audio_url.includes('schedule')) {
    return 'Move my strategy sync to tomorrow at 11 and notify the team.';
  }

  return 'Give me a quick update on my top three priorities today.';
}

function slugify(text: string): string {
  const slug = text
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, '-')
    .replace(/^-+|-+$/g, '')
    .slice(0, 36);
  return slug || 'reply';
}

export async function runClient(input: VocalChatInput): Promise<VocalChatOutput> {
  const transcript = deriveTranscript(input);
  const reply_text = transcript.includes('top three priorities')
    ? 'Here are your top priorities: finalize the board deck, confirm hiring decisions, and close today\'s partner outreach.'
    : 'Understood. I will reschedule the strategy sync for tomorrow at 11 and prepare the team update draft.';

  return {
    provider: 'vocal-chat',
    transcript,
    reply_text,
    reply_audio_url: `https://cdn.brevio.local/voice/vocal-chat/${slugify(reply_text)}.${input.response_voice ?? 'alloy'}.mp3`,
    stt_provider: input.duration_ms > 45000 ? 'gemini-stt' : 'asr',
    tts_provider: 'openai-tts',
    latency_budget_ms: 5000
  };
}

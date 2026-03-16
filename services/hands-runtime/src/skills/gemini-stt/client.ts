import { GoogleGenerativeAI } from '@google/generative-ai';
import type { GeminiSttInput, GeminiSttOutput, GeminiSttSegment } from './types.js';

const FETCH_TIMEOUT_MS  = 30_000;
const MAX_AUDIO_BYTES   = 20 * 1024 * 1024; // 20 MB
const MAX_TRANSCRIPT    = 4096;
const MAX_SEGMENT_TEXT  = 500;
const MAX_SPEAKERS      = 50;

function detectMime(url: string): string {
  const base = url.split('?')[0] ?? url;
  const ext = (base.split('.').pop() ?? '').toLowerCase();
  const map: Record<string, string> = {
    mp3: 'audio/mp3', mpeg: 'audio/mp3',
    wav: 'audio/wav',
    ogg: 'audio/ogg', opus: 'audio/ogg',
    m4a: 'audio/mp4', aac: 'audio/mp4',
  };
  return map[ext] ?? 'audio/mp3';
}

async function fetchAudioBytes(audioUrl: string): Promise<Buffer> {
  const controller = new AbortController();
  const timer = setTimeout(() => controller.abort(), FETCH_TIMEOUT_MS);
  try {
    const resp = await fetch(audioUrl, { signal: controller.signal });
    if (resp.status !== 200) {
      throw new Error(`gemini-stt: audio fetch failed: HTTP ${resp.status} ${audioUrl}`);
    }
    return Buffer.from(await resp.arrayBuffer());
  } catch (err: unknown) {
    if (err instanceof Error && err.name === 'AbortError') {
      throw new Error('gemini-stt: audio fetch timed out after 30s');
    }
    throw err;
  } finally {
    clearTimeout(timer);
  }
}

function buildPrompt(input: GeminiSttInput): string {
  const includeSpeakers = input.include_speaker_labels ?? true;
  const langHint = input.language_hint ?? 'auto-detect';
  return `You are a precise audio transcription and speaker diarisation engine.
Transcribe the provided audio completely and accurately.
${includeSpeakers
  ? 'Identify and label each speaker as "Speaker 1", "Speaker 2", etc.'
  : 'Use a single speaker label "Speaker".'}
Return ONLY valid JSON, no markdown, no explanation:
{
  "transcript": "<full transcript as single string>",
  "language": "<BCP-47 language code e.g. en-US>",
  "confidence": <float 0.0-1.0>,
  "speakers": [
    { "speaker": "<label>", "start_ms": <int>, "end_ms": <int>, "text": "<text>" }
  ]
}
Rules:
- confidence: 0.95+ for clear audio, 0.7-0.94 moderate noise, below 0.7 poor quality
- each speaker segment must be non-empty
- language: detected language, not a guess; use ${langHint} as hint`;
}

function normaliseSegments(raw: unknown[]): GeminiSttSegment[] {
  return (raw.slice(0, MAX_SPEAKERS) as Record<string, unknown>[]).map(seg => {
    const startMs = typeof seg['start_ms'] === 'number' ? Math.max(0, seg['start_ms']) : 0;
    let endMs = typeof seg['end_ms'] === 'number' ? seg['end_ms'] : startMs + 1000;
    if (endMs <= startMs) endMs = startMs + 1000;
    const rawText = typeof seg['text'] === 'string' ? seg['text'] : '';
    const text = rawText.slice(0, MAX_SEGMENT_TEXT) || '[inaudible]';
    return {
      speaker:  typeof seg['speaker'] === 'string' ? seg['speaker'] : 'Speaker',
      start_ms: startMs,
      end_ms:   endMs,
      text,
    };
  });
}

export interface GeminiModel {
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  generateContent(parts: any): Promise<{ response: { text(): string } }>;
}

export interface RunClientDeps {
  createModel?: (apiKey: string) => GeminiModel;
}

function defaultCreateModel(apiKey: string): GeminiModel {
  const genAI = new GoogleGenerativeAI(apiKey);
  return genAI.getGenerativeModel({
    model: 'gemini-2.5-flash',
    generationConfig: { responseMimeType: 'application/json', temperature: 0 },
  });
}

export async function runClient(input: GeminiSttInput, deps: RunClientDeps = {}): Promise<GeminiSttOutput> {
  const apiKey = process.env.GEMINI_API_KEY;
  if (!apiKey) throw new Error('gemini-stt: GEMINI_API_KEY env var is not set');

  const audio = await fetchAudioBytes(input.audio_url);
  if (audio.byteLength > MAX_AUDIO_BYTES) {
    throw new Error('gemini-stt: audio exceeds 20 MB limit');
  }

  const model = (deps.createModel ?? defaultCreateModel)(apiKey);

  let rawText: string;
  try {
    const result = await model.generateContent([
      { inlineData: { mimeType: detectMime(input.audio_url), data: audio.toString('base64') } },
      buildPrompt(input),
    ]);
    rawText = result.response.text();
  } catch (err: unknown) {
    const msg = err instanceof Error ? err.message : String(err);
    throw new Error('gemini-stt: Gemini API call failed: ' + msg);
  }

  const clean = rawText.replace(/```json/g, '').replace(/```/g, '').trim();
  let parsed: unknown;
  try {
    parsed = JSON.parse(clean);
  } catch {
    throw new Error('gemini-stt: response was not valid JSON: ' + clean.slice(0, 200));
  }

  const p = parsed as Record<string, unknown>;
  for (const field of ['transcript', 'language', 'confidence', 'speakers']) {
    if (!(field in p)) throw new Error(`gemini-stt: response missing required field: ${field}`);
  }
  if (typeof p['transcript'] !== 'string' || !p['transcript']) {
    throw new Error('gemini-stt: response missing required field: transcript');
  }
  if (typeof p['confidence'] !== 'number') {
    throw new Error('gemini-stt: response missing required field: confidence');
  }
  if (!Array.isArray(p['speakers']) || (p['speakers'] as unknown[]).length === 0) {
    throw new Error('gemini-stt: response missing required field: speakers');
  }

  const rawTx = (p['transcript'] as string).slice(0, MAX_TRANSCRIPT);
  const transcript = rawTx.length === MAX_TRANSCRIPT
    ? rawTx.slice(0, MAX_TRANSCRIPT - 3) + '...'
    : rawTx;

  return {
    provider:          'gemini-stt',
    transcript,
    language:          (typeof p['language'] === 'string' ? p['language'] : null) ?? input.language_hint ?? 'en-US',
    confidence:        Math.min(1, Math.max(0, p['confidence'] as number)),
    speakers:          normaliseSegments(p['speakers'] as unknown[]),
    latency_budget_ms: 5000,
  };
}

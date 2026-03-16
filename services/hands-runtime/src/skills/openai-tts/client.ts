import OpenAI from 'openai';
import { S3Client, PutObjectCommand, GetObjectCommand } from '@aws-sdk/client-s3';
import { getSignedUrl } from '@aws-sdk/s3-request-presigner';
import { v4 as uuidv4 } from 'uuid';
import type { OpenAiTtsInput, OpenAiTtsOutput } from './types.js';

/** Dependency-injection seam for testing. Production callers omit this. */
export interface RunClientDeps {
  openaiSpeechCreate?: (params: {
    model: string;
    voice: string;
    input: string;
    response_format: string;
    speed: number;
  }) => Promise<{ arrayBuffer(): Promise<ArrayBuffer> }>;
  s3Send?: (cmd: unknown) => Promise<unknown>;
  s3Presign?: (cmd: unknown, opts: { expiresIn: number }) => Promise<string>;
  generateId?: () => string;
}

const mimeMap: Record<string, string> = {
  mp3: 'audio/mpeg',
  wav: 'audio/wav',
  ogg: 'audio/ogg',
  opus: 'audio/ogg',
};

const bytesPerMs: Record<string, number> = {
  mp3: 3.0, wav: 32.0, ogg: 2.5, opus: 2.5,
};

function requireEnv(name: string): string {
  const val = process.env[name];
  if (!val) throw new Error(`openai-tts: ${name} env var is not set`);
  return val;
}

export async function runClient(input: OpenAiTtsInput, deps: RunClientDeps = {}): Promise<OpenAiTtsOutput> {
  // Step 1 — Validate environment
  const apiKey          = requireEnv('OPENAI_API_KEY');
  const bucket          = requireEnv('OPENAI_TTS_S3_BUCKET');
  const region          = requireEnv('OPENAI_TTS_S3_REGION');
  requireEnv('AWS_ACCESS_KEY_ID');
  requireEnv('AWS_SECRET_ACCESS_KEY');

  // Step 2 — Normalise inputs
  const voice  = input.voice  ?? 'alloy';
  const format = input.format ?? 'mp3';
  const text   = input.text.trim();

  if (!text)            throw new Error('openai-tts: text must not be empty');
  if (text.length > 500) throw new Error('openai-tts: text exceeds 500 character limit');

  // Step 3 — Call OpenAI TTS API
  let audioBuffer: Buffer;
  try {
    if (deps.openaiSpeechCreate) {
      const resp = await deps.openaiSpeechCreate({
        model: 'tts-1',
        voice,
        input: text,
        response_format: format === 'ogg' ? 'opus' : format,
        speed: 1.0,
      });
      audioBuffer = Buffer.from(await resp.arrayBuffer());
    } else {
      const openai = new OpenAI({ apiKey });
      const response = await openai.audio.speech.create({
        model: 'tts-1',
        voice,
        input: text,
        response_format: format === 'ogg' ? 'opus' : format,
        speed: 1.0,
      });
      audioBuffer = Buffer.from(await response.arrayBuffer());
    }
  } catch (err: unknown) {
    if (err instanceof Error && err.message.startsWith('openai-tts:')) throw err;
    const msg = err instanceof Error ? err.message : String(err);
    throw new Error('openai-tts: OpenAI API call failed: ' + msg);
  }

  if (audioBuffer.byteLength === 0) {
    throw new Error('openai-tts: received empty audio response');
  }

  // Step 4 — Upload to S3
  const prefix = (process.env.OPENAI_TTS_S3_PREFIX ?? 'tts/').replace(/\/$/, '');
  const ext    = format === 'ogg' ? 'opus' : format;
  const key    = `${prefix}/${(deps.generateId ?? uuidv4)()}.${ext}`;
  const expiry = parseInt(process.env.OPENAI_TTS_URL_EXPIRY_SEC ?? '3600', 10);

  const s3 = deps.s3Send ? null : new S3Client({
    region,
    credentials: {
      accessKeyId: process.env.AWS_ACCESS_KEY_ID!,
      secretAccessKey: process.env.AWS_SECRET_ACCESS_KEY!,
    },
  });

  try {
    const putCmd = new PutObjectCommand({
      Bucket: bucket,
      Key: key,
      Body: audioBuffer,
      ContentType: mimeMap[ext] ?? 'audio/mpeg',
      CacheControl: 'private, max-age=3600',
    });
    if (deps.s3Send) {
      await deps.s3Send(putCmd);
    } else {
      await s3!.send(putCmd);
    }
  } catch (err: unknown) {
    if (err instanceof Error && err.message.startsWith('openai-tts:')) throw err;
    const msg = err instanceof Error ? err.message : String(err);
    throw new Error('openai-tts: S3 upload failed: ' + msg);
  }

  // Step 5 — Generate presigned URL
  let audioUrl: string;
  try {
    const getCmd = new GetObjectCommand({ Bucket: bucket, Key: key });
    if (deps.s3Presign) {
      audioUrl = await deps.s3Presign(getCmd, { expiresIn: expiry });
    } else {
      audioUrl = await getSignedUrl(s3!, getCmd, { expiresIn: expiry });
    }
  } catch (err: unknown) {
    if (err instanceof Error && err.message.startsWith('openai-tts:')) throw err;
    const msg = err instanceof Error ? err.message : String(err);
    throw new Error('openai-tts: S3 presign failed: ' + msg);
  }

  // Step 6 — Estimate duration
  const estimatedMs = Math.round(audioBuffer.byteLength / (bytesPerMs[ext] ?? 3.0));
  const clampedMs   = Math.max(100, Math.min(estimatedMs, 300_000));

  // Step 7 — Return
  return {
    provider: 'openai-tts',
    voice,
    format,
    audio_url: audioUrl,
    estimated_duration_ms: clampedMs,
    latency_budget_ms: 2000,
  };
}

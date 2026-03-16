import assert from 'node:assert/strict';
import { describe, it, before, after } from 'node:test';
import { runClient } from '../client.js';
import type { OpenAiTtsInput } from '../types.js';
import type { RunClientDeps } from '../client.js';

const PRESIGNED_URL = 'https://s3.amazonaws.com/bucket/tts/test.mp3?sig=abc';

function setRequiredEnv(): void {
  process.env.OPENAI_API_KEY = 'test-key';
  process.env.OPENAI_TTS_S3_BUCKET = 'test-bucket';
  process.env.OPENAI_TTS_S3_REGION = 'us-east-1';
  process.env.AWS_ACCESS_KEY_ID = 'AKIATEST';
  process.env.AWS_SECRET_ACCESS_KEY = 'secrettest';
}

function happyDeps(audioBytes = 9000): RunClientDeps {
  return {
    openaiSpeechCreate: async () => ({
      arrayBuffer: async () => new ArrayBuffer(audioBytes),
    }),
    s3Send: async () => ({}),
    s3Presign: async () => PRESIGNED_URL,
    generateId: () => 'test-uuid',
  };
}

describe('openai-tts client', () => {
  const savedEnv: Record<string, string | undefined> = {};
  const envKeys = [
    'OPENAI_API_KEY', 'OPENAI_TTS_S3_BUCKET', 'OPENAI_TTS_S3_REGION',
    'AWS_ACCESS_KEY_ID', 'AWS_SECRET_ACCESS_KEY', 'OPENAI_TTS_S3_PREFIX',
    'OPENAI_TTS_URL_EXPIRY_SEC',
  ];

  before(() => { for (const k of envKeys) savedEnv[k] = process.env[k]; });
  after(() => {
    for (const k of envKeys) {
      if (savedEnv[k] !== undefined) process.env[k] = savedEnv[k];
      else delete process.env[k];
    }
  });

  it('1. throws when OPENAI_API_KEY not set', async () => {
    delete process.env.OPENAI_API_KEY;
    process.env.OPENAI_TTS_S3_BUCKET = 'b';
    process.env.OPENAI_TTS_S3_REGION = 'us-east-1';
    process.env.AWS_ACCESS_KEY_ID = 'x';
    process.env.AWS_SECRET_ACCESS_KEY = 'x';
    await assert.rejects(
      () => runClient({ text: 'hello' }, happyDeps()),
      /OPENAI_API_KEY env var is not set/,
    );
  });

  it('2. throws when OPENAI_TTS_S3_BUCKET not set', async () => {
    process.env.OPENAI_API_KEY = 'key';
    delete process.env.OPENAI_TTS_S3_BUCKET;
    process.env.OPENAI_TTS_S3_REGION = 'us-east-1';
    process.env.AWS_ACCESS_KEY_ID = 'x';
    process.env.AWS_SECRET_ACCESS_KEY = 'x';
    await assert.rejects(
      () => runClient({ text: 'hello' }, happyDeps()),
      /OPENAI_TTS_S3_BUCKET env var is not set/,
    );
  });

  it('3. throws when text is empty', async () => {
    setRequiredEnv();
    await assert.rejects(
      () => runClient({ text: '   ' }, happyDeps()),
      /text must not be empty/,
    );
  });

  it('4. throws when text exceeds 500 chars', async () => {
    setRequiredEnv();
    await assert.rejects(
      () => runClient({ text: 'a'.repeat(501) }, happyDeps()),
      /text exceeds 500 character limit/,
    );
  });

  it('5. throws on OpenAI API failure', async () => {
    setRequiredEnv();
    const deps: RunClientDeps = {
      ...happyDeps(),
      openaiSpeechCreate: async () => { throw new Error('rate_limit_exceeded'); },
    };
    await assert.rejects(
      () => runClient({ text: 'hello' }, deps),
      /OpenAI API call failed/,
    );
  });

  it('6. throws on S3 upload failure', async () => {
    setRequiredEnv();
    const deps: RunClientDeps = {
      ...happyDeps(100),
      s3Send: async () => { throw new Error('AccessDenied'); },
    };
    await assert.rejects(
      () => runClient({ text: 'hello' }, deps),
      /S3 upload failed/,
    );
  });

  it('7. happy path returns valid OpenAiTtsOutput', async () => {
    setRequiredEnv();
    const out = await runClient({ text: 'Your briefing is ready.' }, happyDeps(9000));
    assert.equal(out.provider, 'openai-tts');
    assert.ok(out.audio_url.startsWith('https://'));
    assert.ok(out.estimated_duration_ms > 0);
    assert.equal(out.latency_budget_ms, 2000);
    assert.equal(out.voice, 'alloy');
    assert.equal(out.format, 'mp3');
  });

  it('8. voice and format override', async () => {
    setRequiredEnv();
    const input: OpenAiTtsInput = { text: 'hello', voice: 'sage', format: 'wav' };
    const out = await runClient(input, happyDeps(32000));
    assert.equal(out.voice, 'sage');
    assert.equal(out.format, 'wav');
  });
});

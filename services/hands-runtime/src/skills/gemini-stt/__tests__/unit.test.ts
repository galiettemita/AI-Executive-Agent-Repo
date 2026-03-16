import assert from 'node:assert/strict';
import { describe, it, before, after } from 'node:test';
import { runClient } from '../client.js';
import type { GeminiSttInput, GeminiSttOutput } from '../types.js';
import type { RunClientDeps, GeminiModel } from '../client.js';

const validInput: GeminiSttInput = {
  audio_url: 'https://cdn.example.com/audio/meeting.mp3',
  duration_ms: 45000,
  include_speaker_labels: true,
};

const goodResponse = JSON.stringify({
  transcript: 'Hi, can you send the report by Friday?',
  language: 'en-US',
  confidence: 0.96,
  speakers: [
    { speaker: 'Speaker 1', start_ms: 0,    end_ms: 2000, text: 'Hi,' },
    { speaker: 'Speaker 2', start_ms: 2000, end_ms: 5000, text: 'can you send the report by Friday?' },
  ],
});

function stubFetch(status: number, bytes: number): void {
  (globalThis as Record<string, unknown>)['fetch'] = async () => ({
    status,
    arrayBuffer: async () => new ArrayBuffer(bytes),
  });
}

function fakeModel(responseText: string): RunClientDeps {
  return {
    createModel: () => ({
      generateContent: async () => ({
        response: { text: () => responseText },
      }),
    }),
  };
}

describe('gemini-stt client', () => {
  let savedKey: string | undefined;
  let savedFetch: unknown;
  before(() => { savedKey = process.env.GEMINI_API_KEY; savedFetch = globalThis.fetch; });
  after(() => {
    if (savedKey !== undefined) process.env.GEMINI_API_KEY = savedKey;
    else delete process.env.GEMINI_API_KEY;
    (globalThis as Record<string, unknown>)['fetch'] = savedFetch;
  });

  it('1. throws when GEMINI_API_KEY not set', async () => {
    delete process.env.GEMINI_API_KEY;
    await assert.rejects(() => runClient(validInput), /GEMINI_API_KEY env var is not set/);
  });

  it('2. throws when audio fetch returns non-200', async () => {
    process.env.GEMINI_API_KEY = 'test';
    stubFetch(404, 0);
    await assert.rejects(() => runClient(validInput, fakeModel(goodResponse)), /audio fetch failed: HTTP 404/);
  });

  it('3. throws when audio exceeds 20 MB', async () => {
    process.env.GEMINI_API_KEY = 'test';
    stubFetch(200, 21 * 1024 * 1024);
    await assert.rejects(() => runClient(validInput, fakeModel(goodResponse)), /audio exceeds 20 MB limit/);
  });

  it('4. throws when Gemini returns malformed JSON', async () => {
    process.env.GEMINI_API_KEY = 'test';
    stubFetch(200, 1000);
    await assert.rejects(() => runClient(validInput, fakeModel('not json at all')), /response was not valid JSON/);
  });

  it('5. throws when response missing confidence field', async () => {
    process.env.GEMINI_API_KEY = 'test';
    stubFetch(200, 1000);
    const noConf = JSON.stringify({
      transcript: 'hello', language: 'en-US',
      speakers: [{ speaker: 'S1', start_ms: 0, end_ms: 1000, text: 'hello' }],
    });
    await assert.rejects(() => runClient(validInput, fakeModel(noConf)), /missing required field/);
  });

  it('6. happy path returns valid GeminiSttOutput', async () => {
    process.env.GEMINI_API_KEY = 'test';
    stubFetch(200, 5000);
    const out = await runClient(validInput, fakeModel(goodResponse));
    assert.equal(out.provider, 'gemini-stt');
    assert.ok(out.confidence >= 0 && out.confidence <= 1);
    assert.ok(Array.isArray(out.speakers) && out.speakers.length >= 1);
    assert.ok(out.transcript.length >= 1);
    assert.equal(out.latency_budget_ms, 5000);
  });

  it('7. single speaker when include_speaker_labels false', async () => {
    process.env.GEMINI_API_KEY = 'test';
    stubFetch(200, 1000);
    const resp = JSON.stringify({
      transcript: 'hello', language: 'en-US', confidence: 0.9,
      speakers: [{ speaker: 'Speaker', start_ms: 0, end_ms: 2000, text: 'hello' }],
    });
    const out = await runClient({ ...validInput, include_speaker_labels: false }, fakeModel(resp));
    assert.equal(out.speakers[0]?.speaker, 'Speaker');
  });

  it('8. confidence clamped to 1.0 when Gemini returns 1.5', async () => {
    process.env.GEMINI_API_KEY = 'test';
    stubFetch(200, 1000);
    const resp = JSON.stringify({
      transcript: 'hi', language: 'en-US', confidence: 1.5,
      speakers: [{ speaker: 'Speaker 1', start_ms: 0, end_ms: 500, text: 'hi' }],
    });
    const out = await runClient(validInput, fakeModel(resp));
    assert.equal(out.confidence, 1.0);
  });

  it('9. transcript truncated to 4096 chars with ellipsis', async () => {
    process.env.GEMINI_API_KEY = 'test';
    stubFetch(200, 1000);
    const resp = JSON.stringify({
      transcript: 'x'.repeat(5000), language: 'en-US', confidence: 0.9,
      speakers: [{ speaker: 'S1', start_ms: 0, end_ms: 5000, text: 'x' }],
    });
    const out = await runClient(validInput, fakeModel(resp));
    assert.ok(out.transcript.length <= 4096);
    assert.ok(out.transcript.endsWith('...'));
  });
});

import { describe, it } from 'node:test';
import assert from 'node:assert/strict';
import { MessageEnvelopeSchema, MessageEnvelopeJsonSchema } from '../schemas/message-envelope.js';
import { SkillResultSchema, SkillResultJsonSchema } from '../schemas/skill-result.js';

const VALID_UUID = '550e8400-e29b-41d4-a716-446655440000';
const VALID_HASH = 'a'.repeat(64);

function validEnvelope() {
  return {
    id: VALID_UUID,
    channel: 'WHATSAPP' as const,
    user_id: VALID_UUID,
    timestamp: '2026-03-13T00:00:00Z',
    content: { type: 'TEXT' as const, text: 'hello' },
    metadata: {
      channel_message_id: 'msg-1',
      session_id: VALID_UUID,
    },
    context: {
      user_profile_hash: VALID_HASH,
    },
  };
}

function validSkillResult() {
  return {
    skill_id: 'calendar.create',
    status: 'SUCCESS' as const,
    latency_ms: 120,
    metadata: {
      retries: 0,
      circuit_breaker_state: 'CLOSED' as const,
      cache_hit: false,
    },
  };
}

describe('MessageEnvelopeSchema', () => {
  it('accepts a valid envelope', () => {
    const result = MessageEnvelopeSchema.safeParse(validEnvelope());
    assert.equal(result.success, true);
  });

  it('accepts an envelope with optional routing', () => {
    const env = { ...validEnvelope(), routing: { intent: 'schedule_meeting' } };
    const result = MessageEnvelopeSchema.safeParse(env);
    assert.equal(result.success, true);
  });

  it('rejects when id is missing', () => {
    const { id: _, ...rest } = validEnvelope();
    const result = MessageEnvelopeSchema.safeParse(rest);
    assert.equal(result.success, false);
  });

  it('rejects when id is not a uuid', () => {
    const env = { ...validEnvelope(), id: 'not-a-uuid' };
    const result = MessageEnvelopeSchema.safeParse(env);
    assert.equal(result.success, false);
  });

  it('rejects an invalid channel', () => {
    const env = { ...validEnvelope(), channel: 'EMAIL' };
    const result = MessageEnvelopeSchema.safeParse(env);
    assert.equal(result.success, false);
  });

  it('rejects when content is missing', () => {
    const { content: _, ...rest } = validEnvelope();
    const result = MessageEnvelopeSchema.safeParse(rest);
    assert.equal(result.success, false);
  });

  it('rejects when metadata is missing', () => {
    const { metadata: _, ...rest } = validEnvelope();
    const result = MessageEnvelopeSchema.safeParse(rest);
    assert.equal(result.success, false);
  });

  it('rejects when context.user_profile_hash is wrong length', () => {
    const env = validEnvelope();
    env.context.user_profile_hash = 'tooshort';
    const result = MessageEnvelopeSchema.safeParse(env);
    assert.equal(result.success, false);
  });

  it('rejects an invalid content type', () => {
    const env = validEnvelope();
    (env.content as any).type = 'VIDEO';
    const result = MessageEnvelopeSchema.safeParse(env);
    assert.equal(result.success, false);
  });

  it('rejects when timestamp is not ISO datetime', () => {
    const env = { ...validEnvelope(), timestamp: 'yesterday' };
    const result = MessageEnvelopeSchema.safeParse(env);
    assert.equal(result.success, false);
  });
});

describe('SkillResultSchema', () => {
  it('accepts a valid skill result', () => {
    const result = SkillResultSchema.safeParse(validSkillResult());
    assert.equal(result.success, true);
  });

  it('accepts a result with optional error block', () => {
    const sr = {
      ...validSkillResult(),
      status: 'FAILED' as const,
      error: {
        code: 'EXTERNAL_TIMEOUT' as const,
        message: 'upstream timed out',
        retryable: true,
        http_status: 504,
      },
    };
    const result = SkillResultSchema.safeParse(sr);
    assert.equal(result.success, true);
  });

  it('accepts optional tokens_used and cost_cents', () => {
    const sr = { ...validSkillResult(), tokens_used: 350, cost_cents: 0.42 };
    const result = SkillResultSchema.safeParse(sr);
    assert.equal(result.success, true);
  });

  it('rejects an invalid status value', () => {
    const sr = { ...validSkillResult(), status: 'UNKNOWN' };
    const result = SkillResultSchema.safeParse(sr);
    assert.equal(result.success, false);
  });

  it('rejects when skill_id is missing', () => {
    const { skill_id: _, ...rest } = validSkillResult();
    const result = SkillResultSchema.safeParse(rest);
    assert.equal(result.success, false);
  });

  it('rejects when latency_ms is missing', () => {
    const { latency_ms: _, ...rest } = validSkillResult();
    const result = SkillResultSchema.safeParse(rest);
    assert.equal(result.success, false);
  });

  it('rejects when metadata is missing', () => {
    const { metadata: _, ...rest } = validSkillResult();
    const result = SkillResultSchema.safeParse(rest);
    assert.equal(result.success, false);
  });

  it('rejects an invalid circuit_breaker_state', () => {
    const sr = validSkillResult();
    (sr.metadata as any).circuit_breaker_state = 'BROKEN';
    const result = SkillResultSchema.safeParse(sr);
    assert.equal(result.success, false);
  });

  it('rejects an invalid error code', () => {
    const sr = {
      ...validSkillResult(),
      error: {
        code: 'INVALID_CODE',
        message: 'bad',
        retryable: false,
        http_status: 500,
      },
    };
    const result = SkillResultSchema.safeParse(sr);
    assert.equal(result.success, false);
  });
});

describe('JSON schema exports', () => {
  it('MessageEnvelopeJsonSchema is a valid JSON Schema object', () => {
    assert.equal(typeof MessageEnvelopeJsonSchema, 'object');
    assert.ok(MessageEnvelopeJsonSchema !== null);
    assert.equal(MessageEnvelopeJsonSchema.$schema, 'http://json-schema.org/draft-07/schema#');
    assert.equal(MessageEnvelopeJsonSchema.type, 'object');
    assert.ok(MessageEnvelopeJsonSchema.properties);
  });

  it('SkillResultJsonSchema is a valid JSON Schema object', () => {
    assert.equal(typeof SkillResultJsonSchema, 'object');
    assert.ok(SkillResultJsonSchema !== null);
    assert.equal(SkillResultJsonSchema.$schema, 'http://json-schema.org/draft-07/schema#');
    assert.equal(SkillResultJsonSchema.type, 'object');
    assert.ok(SkillResultJsonSchema.properties);
  });

  it('MessageEnvelopeJsonSchema lists expected required fields', () => {
    assert.ok(Array.isArray(MessageEnvelopeJsonSchema.required));
    const required = MessageEnvelopeJsonSchema.required!;
    for (const field of ['id', 'channel', 'user_id', 'timestamp', 'content', 'metadata', 'context']) {
      assert.ok(required.includes(field), `missing required field: ${field}`);
    }
  });

  it('SkillResultJsonSchema lists expected required fields', () => {
    assert.ok(Array.isArray(SkillResultJsonSchema.required));
    const required = SkillResultJsonSchema.required!;
    for (const field of ['skill_id', 'status', 'latency_ms', 'metadata']) {
      assert.ok(required.includes(field), `missing required field: ${field}`);
    }
  });
});

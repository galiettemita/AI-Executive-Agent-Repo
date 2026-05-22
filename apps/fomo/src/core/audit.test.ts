import assert from 'node:assert/strict';
import { describe, it } from 'node:test';

import { InMemoryAuditStore } from './audit.ts';

describe('InMemoryAuditStore', () => {
  it('records and reads back recent entries for a user', async () => {
    const store = new InMemoryAuditStore();
    await store.write({
      actor_user_id: 'u1',
      actor_ip: null,
      actor_user_agent: null,
      action: 'consent.grant',
      target: 'category:email',
      result: 'success',
      detail: { source: 'inline_prompt' }
    });
    await store.write({
      actor_user_id: 'u1',
      actor_ip: null,
      actor_user_agent: null,
      action: 'oauth.connect',
      target: 'provider:google',
      result: 'success',
      detail: null
    });
    const out = await store.recent('u1');
    assert.equal(out.length, 2);
    assert.equal(out[0]?.action, 'oauth.connect'); // most recent first
    assert.equal(out[1]?.action, 'consent.grant');
  });

  it('isolates entries per user', async () => {
    const store = new InMemoryAuditStore();
    await store.write({
      actor_user_id: 'u1', actor_ip: null, actor_user_agent: null,
      action: 'consent.grant', target: 'category:email', result: 'success', detail: null
    });
    await store.write({
      actor_user_id: 'u2', actor_ip: null, actor_user_agent: null,
      action: 'consent.grant', target: 'category:money', result: 'success', detail: null
    });
    const u1 = await store.recent('u1');
    const u2 = await store.recent('u2');
    assert.equal(u1.length, 1);
    assert.equal(u2.length, 1);
    assert.equal(u1[0]?.target, 'category:email');
    assert.equal(u2[0]?.target, 'category:money');
  });

  it('redacts sensitive fields in detail before persisting', async () => {
    const store = new InMemoryAuditStore();
    await store.write({
      actor_user_id: 'u1', actor_ip: null, actor_user_agent: null,
      action: 'oauth.connect', target: 'provider:google', result: 'success',
      detail: { access_token: 'plaintext', scope_count: 2 }
    });
    const [entry] = await store.recent('u1');
    assert.equal((entry?.detail as Record<string, unknown>).access_token, '<redacted>');
    assert.equal((entry?.detail as Record<string, unknown>).scope_count, 2);
  });

  it('respects capacity limit', async () => {
    const store = new InMemoryAuditStore(3);
    for (let i = 0; i < 5; i++) {
      await store.write({
        actor_user_id: 'u1', actor_ip: null, actor_user_agent: null,
        action: 'consent.grant', target: `i:${i}`, result: 'success', detail: null
      });
    }
    const out = await store.recent('u1');
    assert.equal(out.length, 3);
  });
});

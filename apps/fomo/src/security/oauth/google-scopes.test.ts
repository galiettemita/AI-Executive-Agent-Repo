// Phase v0.6.0C — Google OAuth scope-list helper truth table.
//
// Covers PASS criterion C12:
//   FOMO_CALENDAR_CONTEXT_ENABLED=false → [gmail.readonly] only
//   FOMO_CALENDAR_CONTEXT_ENABLED=true  → [gmail.readonly, calendar.events.readonly]

import assert from 'node:assert/strict';
import { describe, it } from 'node:test';

import { GMAIL_READONLY_SCOPE } from '../../adapters/gmail/client.ts';
import { CALENDAR_EVENTS_READONLY_SCOPE } from '../../adapters/google-calendar/client.ts';
import { googleAuthorizeScopes } from './google-scopes.ts';

describe('googleAuthorizeScopes — truth table', () => {
  it('OFF: returns [gmail.readonly] only (bit-identical to v0.5.x baseline)', () => {
    const scopes = googleAuthorizeScopes(false);
    assert.deepEqual(Array.from(scopes), [GMAIL_READONLY_SCOPE]);
  });

  it('ON: returns [gmail.readonly, calendar.events.readonly]', () => {
    const scopes = googleAuthorizeScopes(true);
    assert.deepEqual(Array.from(scopes), [
      GMAIL_READONLY_SCOPE,
      CALENDAR_EVENTS_READONLY_SCOPE
    ]);
  });

  it('returned arrays are frozen (cannot be mutated by callers)', () => {
    const scopes = googleAuthorizeScopes(true);
    assert.equal(Object.isFrozen(scopes), true);
  });

  it('OFF never includes the Calendar scope (defense-in-depth)', () => {
    const scopes = Array.from(googleAuthorizeScopes(false));
    assert.equal(scopes.includes(CALENDAR_EVENTS_READONLY_SCOPE), false);
  });
});

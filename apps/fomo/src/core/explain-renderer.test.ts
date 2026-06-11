// Phase v0.7.0A — Explain renderer unit tests.

import assert from 'node:assert/strict';
import { describe, it } from 'node:test';

import {
  EMPTY_STATE_COPY,
  EXPLAIN_BODY_MAX_CHARS,
  EXPLAIN_LOOKUP_WINDOW_MS,
  EXPLAIN_SOURCE_SURFACE,
  EXPLAIN_TEMPLATE_VERSION,
  composeExplanationFromReason,
  hashAlertIdForAudit,
  truncateAtWordBoundary
} from './explain-renderer.ts';

describe('composeExplanationFromReason — natural-prose composition', () => {
  it('preserves proper-noun starters and adds the I-flagged frame', () => {
    const out = composeExplanationFromReason(
      'Mark needs you to review the Q3 board deck tonight.'
    );
    assert.equal(
      out,
      'I flagged it because Mark needs you to review the Q3 board deck tonight.'
    );
  });

  it('lowercases common pronoun starters so the sentence reads naturally', () => {
    const out = composeExplanationFromReason(
      'Your CEO is asking for a response by 5pm.'
    );
    assert.equal(
      out,
      'I flagged it because your CEO is asking for a response by 5pm.'
    );
  });

  it('lowercases "You" at the start', () => {
    const out = composeExplanationFromReason(
      "You're being asked to confirm the meeting."
    );
    assert.equal(
      out,
      "I flagged it because you're being asked to confirm the meeting."
    );
  });

  it('lowercases "The" article at the start', () => {
    const out = composeExplanationFromReason(
      'The investor is asking for the deck before Friday.'
    );
    assert.equal(
      out,
      'I flagged it because the investor is asking for the deck before Friday.'
    );
  });

  it('strips trailing period before composing (no double period)', () => {
    const out = composeExplanationFromReason('Galiette needs the deck.');
    assert.equal(out, 'I flagged it because Galiette needs the deck.');
    assert.equal(out?.includes('..'), false);
  });

  it('strips trailing exclamation and multi-punctuation', () => {
    assert.equal(
      composeExplanationFromReason('Galiette is asking!'),
      'I flagged it because Galiette is asking.'
    );
    assert.equal(
      composeExplanationFromReason('Why now??'),
      'I flagged it because Why now.'
    );
    assert.equal(
      composeExplanationFromReason('Critical...'),
      'I flagged it because Critical.'
    );
  });

  it('returns null for empty / whitespace-only reason', () => {
    assert.equal(composeExplanationFromReason(''), null);
    assert.equal(composeExplanationFromReason('   '), null);
    assert.equal(composeExplanationFromReason('\n\t '), null);
  });

  it('returns null for non-string input', () => {
    assert.equal(composeExplanationFromReason(null), null);
    assert.equal(composeExplanationFromReason(undefined), null);
  });

  it('returns null when the reason was JUST trailing terminal punctuation', () => {
    assert.equal(composeExplanationFromReason('...'), null);
    assert.equal(composeExplanationFromReason('!?!'), null);
  });

  it('output is always under or equal to the 320-char hard cap', () => {
    const longReason =
      'The very important conversation about the very important contract continues across many sentences and the user really needs to know that this email is significant for many specific reasons including a deadline tonight a follow-up tomorrow and a quarterly review on Friday.';
    const out = composeExplanationFromReason(longReason);
    assert.notEqual(out, null);
    assert.ok((out as string).length <= EXPLAIN_BODY_MAX_CHARS);
  });

  it('long reason is truncated at a word boundary, no ellipsis', () => {
    const longReason = 'A'.repeat(50) + ' ' + 'word '.repeat(80);
    const out = composeExplanationFromReason(longReason);
    assert.notEqual(out, null);
    assert.equal((out as string).includes('…'), false);
    assert.equal((out as string).includes('...'), false);
    assert.ok((out as string).endsWith('.'));
  });

  it('never produces a field-shaped output (no "From X" labels, no quotes, no em-dashes)', () => {
    const reasons = [
      'Mark needs you to review the deck tonight.',
      'Your CEO is asking for a response.',
      'Galiette is requesting an urgent reply.'
    ];
    for (const r of reasons) {
      const out = composeExplanationFromReason(r);
      const s = out as string;
      assert.equal(/^From /.test(s), false, `field-shaped "From" detected in: ${s}`);
      assert.equal(s.includes('—'), false, `em-dash detected in: ${s}`);
      // No surrounding double-quotes (subject-as-label pattern).
      assert.equal(/"[^"]*"/.test(s), false, `quoted label detected in: ${s}`);
    }
  });

  it('preserves apostrophes in proper-noun starters and possessives', () => {
    const out = composeExplanationFromReason("Galiette's team needs the deck.");
    assert.equal(out, "I flagged it because Galiette's team needs the deck.");
  });
});

describe('truncateAtWordBoundary', () => {
  it('returns the string unchanged when under the limit', () => {
    assert.equal(truncateAtWordBoundary('short string', 100), 'short string');
  });

  it('truncates at the last space within the limit', () => {
    assert.equal(truncateAtWordBoundary('one two three four five', 12), 'one two');
  });

  it('hard-cuts when no good word boundary exists in the limit', () => {
    const longRun = 'x'.repeat(50);
    assert.equal(truncateAtWordBoundary(longRun, 10), 'x'.repeat(10));
  });

  it('returns the string unchanged when exactly at the limit', () => {
    const s = 'a'.repeat(10);
    assert.equal(truncateAtWordBoundary(s, 10), s);
  });
});

describe('hashAlertIdForAudit', () => {
  it('returns null when alert_id is null', () => {
    assert.equal(hashAlertIdForAudit(null), null);
  });

  it('returns deterministic 16 hex chars for the same alert_id', () => {
    const a = hashAlertIdForAudit('alert-uuid-12345');
    const b = hashAlertIdForAudit('alert-uuid-12345');
    assert.equal(a, b);
    assert.notEqual(a, null);
    assert.match(a as string, /^[a-f0-9]{16}$/);
  });

  it('different alert_ids produce different hashes', () => {
    const a = hashAlertIdForAudit('alert-A');
    const b = hashAlertIdForAudit('alert-B');
    assert.notEqual(a, b);
  });

  it('hash never contains the raw alert_id substring', () => {
    const alertId = 'alert-secret-token-abc';
    const h = hashAlertIdForAudit(alertId);
    assert.notEqual(h, null);
    assert.equal((h as string).includes('alert'), false);
    assert.equal((h as string).includes('secret'), false);
  });
});

describe('locked constants', () => {
  it('EXPLAIN_TEMPLATE_VERSION matches the v0.7.0A literal', () => {
    assert.equal(EXPLAIN_TEMPLATE_VERSION, 'brevio-explain-v0.1.0');
  });

  it('EMPTY_STATE_COPY matches the founder-locked literal', () => {
    assert.equal(EMPTY_STATE_COPY, "I haven't sent you an alert I can explain yet.");
  });

  it('EMPTY_STATE_COPY fits inside the body cap', () => {
    assert.ok(EMPTY_STATE_COPY.length <= EXPLAIN_BODY_MAX_CHARS);
  });

  it('EXPLAIN_BODY_MAX_CHARS = 320', () => {
    assert.equal(EXPLAIN_BODY_MAX_CHARS, 320);
  });

  it('EXPLAIN_LOOKUP_WINDOW_MS = 24h', () => {
    assert.equal(EXPLAIN_LOOKUP_WINDOW_MS, 24 * 60 * 60 * 1000);
  });

  it('EXPLAIN_SOURCE_SURFACE is the literal sendblue_inbound', () => {
    assert.equal(EXPLAIN_SOURCE_SURFACE, 'sendblue_inbound');
  });
});

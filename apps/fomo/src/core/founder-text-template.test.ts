// Phase v0.5.6 renderFounderText test suite.
//
// Replaces the original 3E.1 test file (which asserted the
// "FOMO · IMPORTANT (0.92)" telemetry header, body_snippet usage, and
// ellipsis truncation — all forbidden by the v0.5.6 founder Q4 lock).
//
// What's covered:
//   - Shape: NO telemetry header, sentence-shaped, NO ellipsis from
//     truncation, NO mid-sentence cuts, NO body_snippet anywhere
//   - Substitution: rank.reason appears in the rendered body
//   - Version: FOUNDER_TEXT_TEMPLATE_VERSION === 'founder-text-v0.2.0'
//   - Length: target 220-280, hard cap 320, absolute cap 340
//   - Fallback: empty/too-long reason triggers deterministic fallback +
//     reason_source='fallback' + reason_violation_kind
//   - Leak discipline: sender_email_masked stays masked, body_snippet
//     never appears, message_id / received_at never appear

import assert from 'node:assert/strict';
import { describe, it } from 'node:test';

import {
  FOUNDER_TEXT_TEMPLATE_VERSION,
  FOUNDER_TEXT_TARGET_MIN_CHARS,
  FOUNDER_TEXT_TARGET_MAX_CHARS,
  FOUNDER_TEXT_HARD_MAX_CHARS,
  FOUNDER_TEXT_ABSOLUTE_MAX_CHARS,
  REASON_FALLBACK_STRING,
  renderFounderText
} from './founder-text-template.ts';
import { type SlackEgressView } from './egress-policy.ts';

function viewFixture(partial: Partial<SlackEgressView> = {}): SlackEgressView {
  return Object.freeze({
    purpose: 'slack_founder_card' as const,
    sender_email_masked: 's***@school.edu',
    sender_name: 'Sarah J.',
    subject: 'Reminder: deposit due tonight',
    body_snippet:
      'Hi! Just a friendly reminder your dorm deposit is due by midnight.',
    received_at: '2026-05-25T12:00:00.000Z',
    message_id: 'msg-abc',
    ...partial
  });
}

const DEFAULT_REASON =
  'Counselor flagged a time-sensitive dorm deposit deadline tonight.';

describe('renderFounderText — shape + version (v0.5.6)', () => {
  it('returns text + template_version + reason_source + reason_violation_kind + original_reason_length', () => {
    const out = renderFounderText({
      view: viewFixture(),
      rank: { label: 'important', score: 0.91, reason: DEFAULT_REASON }
    });
    assert.equal(typeof out.text, 'string');
    assert.equal(out.template_version, FOUNDER_TEXT_TEMPLATE_VERSION);
    assert.equal(out.reason_source, 'rank');
    assert.equal(out.reason_violation_kind, null);
    assert.equal(out.original_reason_length, DEFAULT_REASON.length);
  });

  it('template version is the bumped v0.5.7 HMR rename ("human-message-v0.3.0")', () => {
    // v0.5.7 Q6.A lock — renamed from 'founder-text-v0.2.0' to surface
    // the Human Message Renderer product principle in audit detail.
    assert.equal(FOUNDER_TEXT_TEMPLATE_VERSION, 'human-message-v0.3.0');
  });

  it('output is frozen', () => {
    const out = renderFounderText({
      view: viewFixture(),
      rank: { label: 'important', score: 0.91, reason: DEFAULT_REASON }
    });
    assert.throws(() => {
      (out as unknown as { text: string }).text = 'mutated';
    });
  });
});

describe('renderFounderText — Friend B "robotic" fixes (v0.5.6 Q1–Q3)', () => {
  it('does NOT include the "FOMO · IMPORTANT (0.92)" telemetry header', () => {
    const out = renderFounderText({
      view: viewFixture(),
      rank: { label: 'important', score: 0.91, reason: DEFAULT_REASON }
    });
    assert.doesNotMatch(out.text, /^FOMO ·/);
    assert.doesNotMatch(out.text, /FOMO ·/);
    assert.doesNotMatch(out.text, /IMPORTANT \(0\./);
    assert.doesNotMatch(out.text, /NOT_IMPORTANT/);
  });

  it('does NOT include the numeric ranker score anywhere in the body', () => {
    const out = renderFounderText({
      view: viewFixture(),
      rank: { label: 'important', score: 0.91, reason: DEFAULT_REASON }
    });
    assert.doesNotMatch(out.text, /0\.91/);
    assert.doesNotMatch(out.text, /\(0\./);
  });

  it('substitutes rank.reason as the prose-y line (NOT body_snippet)', () => {
    const view = viewFixture({
      body_snippet:
        'Hey, just bumping this. Did you see the previous email about the deposit?'
    });
    const out = renderFounderText({
      view,
      rank: { label: 'important', score: 0.91, reason: DEFAULT_REASON }
    });
    assert.ok(
      out.text.includes(DEFAULT_REASON),
      `Body must contain rank.reason content; got: ${out.text}`
    );
    assert.ok(
      !out.text.includes('previous email about the deposit'),
      `Body must NOT contain body_snippet content; got: ${out.text}`
    );
  });

  it('composes a Q1.A two-sentence body: "<Sender> emailed you about <subject>. <Why>." (v0.5.7)', () => {
    // v0.5.7 Q1.A canonical template. The output is a SINGLE LINE
    // (no newlines), shaped as two sentences. Sender opener uses the
    // resolved first name (Modified Q2.B step 1) and subject is wrapped
    // in quotes per Q3.B (no aggressive noun rewriting).
    const out = renderFounderText({
      view: viewFixture(),
      rank: { label: 'important', score: 0.91, reason: DEFAULT_REASON }
    });
    // No newlines — the v0.5.6 newline-separated field shape is gone.
    assert.equal(out.text.split('\n').length, 1, `v0.5.7 body must be single-line; got: ${out.text}`);
    // Sender opener (first-name from sender_name = "Sarah J." → "Sarah").
    assert.ok(out.text.startsWith('Sarah emailed you about '), `expected "Sarah emailed you about" opener; got: ${out.text}`);
    // Subject (quoted) — v0.5.7 leaves the subject as-is between quotes.
    assert.ok(out.text.includes('"Reminder: deposit due tonight"'), `subject must appear in quotes; got: ${out.text}`);
    // Reason follows after the period + space.
    assert.ok(out.text.includes(DEFAULT_REASON), `rank.reason content must appear verbatim; got: ${out.text}`);
  });
});

describe('renderFounderText — NO arbitrary ellipsis (v0.5.6 Q4)', () => {
  it('contains zero "…" characters for a normal-length input', () => {
    const out = renderFounderText({
      view: viewFixture(),
      rank: { label: 'important', score: 0.91, reason: DEFAULT_REASON }
    });
    assert.doesNotMatch(out.text, /…/);
    assert.doesNotMatch(out.text, /\.\.\./); // also no three-dot variant
  });

  it('contains zero "…" characters even for an overlong subject', () => {
    const longSubject =
      'A genuinely very long subject line that vastly exceeds the per-line cap and must be truncated cleanly without any arbitrary ellipsis at the end whatsoever';
    const out = renderFounderText({
      view: viewFixture({ subject: longSubject }),
      rank: { label: 'important', score: 0.91, reason: DEFAULT_REASON }
    });
    assert.doesNotMatch(out.text, /…/);
    assert.doesNotMatch(out.text, /\.\.\./);
  });

  it('contains zero "…" characters even for an overlong reason (v0.5.6 fallback path)', () => {
    const longReason = 'x'.repeat(500);
    const out = renderFounderText({
      view: viewFixture(),
      rank: { label: 'important', score: 0.91, reason: longReason }
    });
    assert.doesNotMatch(out.text, /…/);
    assert.doesNotMatch(out.text, /\.\.\./);
  });
});

describe('renderFounderText — length policy (v0.5.7 carry-forward from v0.5.6 Q4)', () => {
  it('overlong subject does not break the body — total stays under hard cap', () => {
    // v0.5.7: Q3.B strip rules apply (bracket/Re:/Fwd: only). The
    // renderer does NOT word-truncate the subject in-place; instead
    // applyLengthBudget shrinks the reason or collapses the shape if
    // the total exceeds HARD_MAX (320). Either way, the output respects
    // the cap and uses no arbitrary ellipsis.
    const longSubject =
      'Reminder: deposit due tonight please confirm or reschedule absolutely by the close of business tomorrow afternoon';
    const out = renderFounderText({
      view: viewFixture({ subject: longSubject }),
      rank: { label: 'important', score: 0.91, reason: DEFAULT_REASON }
    });
    assert.ok(
      out.text.length <= FOUNDER_TEXT_HARD_MAX_CHARS,
      `length ${out.text.length} > HARD_MAX ${FOUNDER_TEXT_HARD_MAX_CHARS}`
    );
    assert.doesNotMatch(out.text, /…/);
  });

  it('reason sentence-boundary truncation: final sentence ends on .!?', () => {
    // v0.5.7: the reason is rendered verbatim (no truncation) when it
    // fits the schema (≤180) and the overall body fits HARD_MAX. The
    // ensureSentenceTerminator helper ensures the body ends on .!?.
    const multiSentenceReason =
      'First sentence about why this matters. Second sentence with more context.';
    const out = renderFounderText({
      view: viewFixture(),
      rank: { label: 'important', score: 0.91, reason: multiSentenceReason }
    });
    assert.match(out.text, /[.!?]$/);
  });
});

describe('renderFounderText — length policy (v0.5.6 Q4)', () => {
  it('a reasonable input stays well under HARD_MAX (320) and ABSOLUTE_MAX (340)', () => {
    const out = renderFounderText({
      view: viewFixture(),
      rank: { label: 'important', score: 0.91, reason: DEFAULT_REASON }
    });
    assert.ok(
      out.text.length <= FOUNDER_TEXT_HARD_MAX_CHARS,
      `length ${out.text.length} > HARD_MAX ${FOUNDER_TEXT_HARD_MAX_CHARS}`
    );
    assert.ok(
      out.text.length <= FOUNDER_TEXT_ABSOLUTE_MAX_CHARS,
      `length ${out.text.length} > ABSOLUTE_MAX ${FOUNDER_TEXT_ABSOLUTE_MAX_CHARS}`
    );
  });

  it('an overlong reason gets truncated so total stays ≤ HARD_MAX (320)', () => {
    const longReason =
      'This is a long but plausible counselor explanation. The dorm deposit is due tonight and the school billing office closes at midnight. Missing it forfeits the room assignment.';
    const out = renderFounderText({
      view: viewFixture({
        subject:
          'Reminder: dorm deposit due tonight by midnight (URGENT, last call)',
        sender_name: 'Sarah Johnson-Smith'
      }),
      rank: { label: 'important', score: 0.91, reason: longReason }
    });
    assert.ok(
      out.text.length <= FOUNDER_TEXT_HARD_MAX_CHARS,
      `length ${out.text.length} > HARD_MAX ${FOUNDER_TEXT_HARD_MAX_CHARS}`
    );
  });

  it('a pathologically overlong reason still respects ABSOLUTE_MAX (340)', () => {
    const out = renderFounderText({
      view: viewFixture(),
      rank: { label: 'important', score: 0.91, reason: 'x '.repeat(800) }
    });
    assert.ok(
      out.text.length <= FOUNDER_TEXT_ABSOLUTE_MAX_CHARS,
      `length ${out.text.length} > ABSOLUTE_MAX ${FOUNDER_TEXT_ABSOLUTE_MAX_CHARS}`
    );
  });

  it('target band (220–280) is realistic for a typical curated alert', () => {
    // Sanity: not a hard test (it's a target, not a cap), but flag if a
    // typical alert ends up grossly under-budget (e.g. < 60 chars) which
    // would suggest the shell composition is broken.
    const out = renderFounderText({
      view: viewFixture(),
      rank: { label: 'important', score: 0.91, reason: DEFAULT_REASON }
    });
    assert.ok(
      out.text.length >= 60,
      `body suspiciously short (${out.text.length} chars); typical alert should be > 60`
    );
    // Document the target band exists for downstream readers.
    assert.equal(FOUNDER_TEXT_TARGET_MIN_CHARS, 220);
    assert.equal(FOUNDER_TEXT_TARGET_MAX_CHARS, 280);
  });
});

describe('renderFounderText — deterministic fallback (v0.5.6 Q6)', () => {
  it('empty reason triggers fallback substitution + reason_source=fallback + reason_violation_kind=empty', () => {
    const out = renderFounderText({
      view: viewFixture(),
      rank: { label: 'important', score: 0.91, reason: '' }
    });
    assert.equal(out.reason_source, 'fallback');
    assert.equal(out.reason_violation_kind, 'empty');
    assert.equal(out.original_reason_length, 0);
    assert.ok(
      out.text.includes(REASON_FALLBACK_STRING),
      `fallback string must appear in body; got: ${out.text}`
    );
  });

  it('whitespace-only reason is treated as empty', () => {
    const out = renderFounderText({
      view: viewFixture(),
      rank: { label: 'important', score: 0.91, reason: '   \n\t  ' }
    });
    assert.equal(out.reason_source, 'fallback');
    assert.equal(out.reason_violation_kind, 'empty');
  });

  it('reason > REASON_HARD_CAP_FOR_RENDER (180) triggers fallback + reason_violation_kind=too_long', () => {
    const tooLong = 'x'.repeat(181);
    const out = renderFounderText({
      view: viewFixture(),
      rank: { label: 'important', score: 0.91, reason: tooLong }
    });
    assert.equal(out.reason_source, 'fallback');
    assert.equal(out.reason_violation_kind, 'too_long');
    assert.equal(out.original_reason_length, 181);
    assert.ok(out.text.includes(REASON_FALLBACK_STRING));
  });

  it('reason at the boundary (=180) does NOT trigger fallback', () => {
    const atBoundary = 'A'.repeat(180);
    const out = renderFounderText({
      view: viewFixture(),
      rank: { label: 'important', score: 0.91, reason: atBoundary }
    });
    assert.equal(out.reason_source, 'rank');
    assert.equal(out.reason_violation_kind, null);
  });

  it('fallback path produces a body within ABSOLUTE_MAX', () => {
    const out = renderFounderText({
      view: viewFixture(),
      rank: { label: 'important', score: 0.91, reason: '' }
    });
    assert.ok(out.text.length <= FOUNDER_TEXT_ABSOLUTE_MAX_CHARS);
    assert.doesNotMatch(out.text, /…/);
  });
});

describe('renderFounderText — leak / privacy discipline (3E.1 carry-forward + v0.5.7 anti-masked)', () => {
  it('NEVER unmasks the sender email (no raw local part in body)', () => {
    const out = renderFounderText({
      view: viewFixture({
        sender_email_masked: 's***@school.edu'
      }),
      rank: { label: 'important', score: 0.91, reason: DEFAULT_REASON }
    });
    // v0.5.6 used to INCLUDE the masked email "s***@school.edu" in the
    // opener. v0.5.7 founder lock 2026-06-06 reverses that — the masked
    // email is NOT in the opener; the Modified Q2.B sender resolution
    // chain uses first-name / domain-label / "Someone" instead.
    assert.ok(!out.text.includes('s***@school.edu'), `v0.5.7 must NOT include masked email in opener; got: ${out.text}`);
    // The opener should use a resolved sender display token instead.
    assert.ok(/emailed you/.test(out.text), `expected "emailed you" opener; got: ${out.text}`);
  });

  it('NEVER includes body_snippet content (v0.5.6: snippet is no longer an input slot)', () => {
    const out = renderFounderText({
      view: viewFixture({
        body_snippet: 'SECRET_CANARY_BODY_SNIPPET_xyz123'
      }),
      rank: { label: 'important', score: 0.91, reason: DEFAULT_REASON }
    });
    assert.ok(!out.text.includes('SECRET_CANARY_BODY_SNIPPET_xyz123'));
    assert.ok(!out.text.includes('canary'));
  });

  it('NEVER includes message_id (opaque identifier, not user-facing copy)', () => {
    const out = renderFounderText({
      view: viewFixture({ message_id: 'msg-canary-abc-123' }),
      rank: { label: 'important', score: 0.91, reason: DEFAULT_REASON }
    });
    assert.ok(!out.text.includes('msg-canary-abc-123'));
    assert.ok(!out.text.includes('msg-'));
  });

  it('NEVER includes received_at (an internal timestamp, not user-facing)', () => {
    const out = renderFounderText({
      view: viewFixture({ received_at: '2026-05-25T12:00:00.000Z' }),
      rank: { label: 'important', score: 0.91, reason: DEFAULT_REASON }
    });
    assert.ok(!out.text.includes('2026-05-25'));
    assert.ok(!out.text.includes('T12:00:00'));
  });

  it('NEVER includes the "From:" / "Subject:" RFC-style email header prefixes', () => {
    const out = renderFounderText({
      view: viewFixture(),
      rank: { label: 'important', score: 0.91, reason: DEFAULT_REASON }
    });
    assert.doesNotMatch(out.text, /^From:/m);
    assert.doesNotMatch(out.text, /^Subject:/m);
  });

  it('when sender_name is missing, uses the Modified Q2.B chain (domain label / "Someone"), NEVER masked email', () => {
    // v0.5.7 founder lock: do NOT expose the masked email as the
    // opener. The Modified Q2.B chain falls through to a domain label
    // (school.edu → "School") for non-curated domains or "Someone" for
    // pathological inputs. Either way, no "***@" in the body.
    const out = renderFounderText({
      view: viewFixture({
        sender_name: '',
        sender_email_masked: 's***@school.edu'
      }),
      rank: { label: 'important', score: 0.91, reason: DEFAULT_REASON }
    });
    assert.ok(!out.text.includes('s***@school.edu'), `must NOT include masked email; got: ${out.text}`);
    assert.ok(!out.text.includes('***@'), `must NOT include any "***@" shape; got: ${out.text}`);
    assert.ok(/emailed you/.test(out.text), `expected "emailed you" opener; got: ${out.text}`);
  });
});

describe('renderFounderText — whitespace collapse', () => {
  it('collapses repeated whitespace inside subject', () => {
    const out = renderFounderText({
      view: viewFixture({ subject: 'foo    bar   baz' }),
      rank: { label: 'important', score: 0.91, reason: DEFAULT_REASON }
    });
    assert.ok(out.text.includes('foo bar baz'));
    assert.ok(!out.text.includes('foo    bar'));
  });

  it('collapses repeated whitespace inside reason', () => {
    const out = renderFounderText({
      view: viewFixture(),
      rank: {
        label: 'important',
        score: 0.91,
        reason: 'counselor    flagged   the    deposit'
      }
    });
    assert.ok(out.text.includes('counselor flagged the deposit'));
    assert.ok(!out.text.includes('counselor    flagged'));
  });
});

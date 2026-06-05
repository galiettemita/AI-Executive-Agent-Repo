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

  it('template version is the bumped v0.2.0', () => {
    assert.equal(FOUNDER_TEXT_TEMPLATE_VERSION, 'founder-text-v0.2.0');
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

  it('contains sender + subject + reason in that order, separated by newlines', () => {
    const out = renderFounderText({
      view: viewFixture(),
      rank: { label: 'important', score: 0.91, reason: DEFAULT_REASON }
    });
    const lines = out.text.split('\n');
    assert.ok(lines[0]?.includes('Sarah J.'), 'first line is sender');
    assert.ok(
      lines[1]?.includes('Reminder: deposit due tonight'),
      'second line is subject'
    );
    assert.ok(
      lines[2]?.includes('Counselor flagged'),
      'third line is rank.reason'
    );
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

describe('renderFounderText — sentence-boundary truncation (v0.5.6 Q4)', () => {
  it('truncates an overlong subject at a word boundary, never mid-word', () => {
    const longSubject =
      'Reminder: deposit due tonight please confirm or reschedule absolutely by the close of business tomorrow afternoon';
    const out = renderFounderText({
      view: viewFixture({ subject: longSubject }),
      rank: { label: 'important', score: 0.91, reason: DEFAULT_REASON }
    });
    const lines = out.text.split('\n');
    const subjectLine = lines[1] ?? '';
    // Load-bearing check: the truncated subject must be a prefix of the
    // collapsed original (no characters added by truncation, no ellipsis)
    // AND the next character in the original after the truncation point
    // must be whitespace OR the truncation must end on a sentence
    // terminator. Either case proves the cut happened at a word boundary,
    // not mid-word.
    assert.doesNotMatch(subjectLine, /…/);
    assert.ok(
      longSubject.startsWith(subjectLine),
      `truncated subject must be a prefix of the original (no chars added); got: "${subjectLine}"`
    );
    const nextChar = longSubject.charAt(subjectLine.length);
    const endsOnSentenceTerminator = /[.!?]$/.test(subjectLine);
    const cutAtWordBoundary = nextChar === '' || /\s/.test(nextChar);
    assert.ok(
      endsOnSentenceTerminator || cutAtWordBoundary,
      `cut must be at a word/sentence boundary; subject="${subjectLine}", nextChar=${JSON.stringify(nextChar)}`
    );
  });

  it('truncates an overlong reason at the last sentence boundary within budget', () => {
    const multiSentenceReason =
      'First sentence about why this matters. Second sentence with more context. Third sentence that pushes past the per-line cap.';
    const out = renderFounderText({
      view: viewFixture(),
      rank: {
        label: 'important',
        score: 0.91,
        reason: multiSentenceReason
      }
    });
    const lines = out.text.split('\n');
    const reasonLine = lines[2] ?? '';
    // The reason line must end on `.` `!` or `?` if any sentence
    // terminator existed in the prefix.
    assert.match(reasonLine, /[.!?]$/);
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

describe('renderFounderText — leak / privacy discipline (3E.1 carry-forward)', () => {
  it('uses the masked sender email (never unmasks)', () => {
    const out = renderFounderText({
      view: viewFixture({
        sender_email_masked: 's***@school.edu'
      }),
      rank: { label: 'important', score: 0.91, reason: DEFAULT_REASON }
    });
    assert.ok(out.text.includes('s***@school.edu'));
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

  it('falls back to masked email alone when sender_name is missing', () => {
    const out = renderFounderText({
      view: viewFixture({
        sender_name: '',
        sender_email_masked: 's***@school.edu'
      }),
      rank: { label: 'important', score: 0.91, reason: DEFAULT_REASON }
    });
    assert.ok(out.text.includes('s***@school.edu'));
    assert.ok(!out.text.includes('<'));
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

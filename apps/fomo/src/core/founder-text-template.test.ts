import assert from 'node:assert/strict';
import { describe, it } from 'node:test';

import {
  FOUNDER_TEXT_TEMPLATE_VERSION,
  renderFounderText
} from './founder-text-template.ts';
import { type SlackEgressView } from './egress-policy.ts';

function viewFixture(partial: Partial<SlackEgressView> = {}): SlackEgressView {
  return Object.freeze({
    purpose: 'slack_founder_card' as const,
    sender_email_masked: 's***@school.edu',
    sender_name: 'Sarah J.',
    subject: 'Reminder: deposit due tonight',
    body_snippet: 'Hi! Just a friendly reminder your dorm deposit is due by midnight.',
    received_at: '2026-05-25T12:00:00.000Z',
    message_id: 'msg-abc',
    ...partial
  });
}

describe('renderFounderText — shape + version', () => {
  it('returns text + template_version', () => {
    const out = renderFounderText({
      view: viewFixture(),
      rank: { label: 'important', score: 0.91 }
    });
    assert.equal(typeof out.text, 'string');
    assert.equal(out.template_version, FOUNDER_TEXT_TEMPLATE_VERSION);
  });

  it('output is frozen', () => {
    const out = renderFounderText({
      view: viewFixture(),
      rank: { label: 'important', score: 0.91 }
    });
    assert.throws(() => {
      (out as unknown as { text: string }).text = 'mutated';
    });
  });

  it('starts with the FOMO label header and includes the score', () => {
    const out = renderFounderText({
      view: viewFixture(),
      rank: { label: 'important', score: 0.91 }
    });
    assert.match(out.text, /^FOMO · IMPORTANT \(0\.91\)/);
  });

  it('renders the NOT_IMPORTANT label when label !== important', () => {
    const out = renderFounderText({
      view: viewFixture(),
      rank: { label: 'not_important', score: 0.12 }
    });
    assert.match(out.text, /^FOMO · NOT_IMPORTANT \(0\.12\)/);
  });

  it('renders score=? when score is not finite', () => {
    const out = renderFounderText({
      view: viewFixture(),
      rank: { label: 'important', score: Number.NaN }
    });
    assert.match(out.text, /^FOMO · IMPORTANT \(\?\)/);
  });
});

describe('renderFounderText — egress-redaction discipline', () => {
  it('uses the masked sender email (does not unmask)', () => {
    const out = renderFounderText({
      view: viewFixture({
        sender_email_masked: 's***@school.edu',
        sender_name: 'Sarah J.'
      }),
      rank: { label: 'important', score: 0.9 }
    });
    assert.ok(out.text.includes('s***@school.edu'));
    assert.ok(!out.text.includes('sarah@school.edu'));
    assert.ok(!out.text.includes('sarah.j@school.edu'));
  });

  it('falls back to masked email alone when sender_name is missing', () => {
    const out = renderFounderText({
      view: viewFixture({ sender_email_masked: 'b***@friend.com', sender_name: undefined }),
      rank: { label: 'important', score: 0.5 }
    });
    assert.ok(out.text.includes('b***@friend.com'));
    assert.ok(!out.text.includes('<'));
  });

  it('never includes message_id (an opaque identifier, but not user-facing copy)', () => {
    const out = renderFounderText({
      view: viewFixture({ message_id: '18f7d3c4b2a1e9d7' }),
      rank: { label: 'important', score: 0.9 }
    });
    assert.ok(!out.text.includes('18f7d3c4b2a1e9d7'));
  });

  it('never includes received_at', () => {
    const out = renderFounderText({
      view: viewFixture({ received_at: '2026-05-25T12:00:00.000Z' }),
      rank: { label: 'important', score: 0.9 }
    });
    assert.ok(!out.text.includes('2026-05-25T12:00:00.000Z'));
  });
});

describe('renderFounderText — bounded length', () => {
  it('stays under 280 characters even for an overlong message', () => {
    const out = renderFounderText({
      view: viewFixture({
        subject: 'X'.repeat(500),
        body_snippet: 'Y'.repeat(500),
        sender_name: 'A really long sender display name '.repeat(20)
      }),
      rank: { label: 'important', score: 0.9 }
    });
    assert.ok(out.text.length <= 280, `len ${out.text.length}`);
  });

  it('drops the snippet first when total length would exceed 280', () => {
    const longSubject = 'Subject text that is fairly long here'.repeat(2);
    const longSnippet = 'Body snippet content that should be the first to drop. '.repeat(10);
    const out = renderFounderText({
      view: viewFixture({
        subject: longSubject,
        body_snippet: longSnippet
      }),
      rank: { label: 'important', score: 0.9 }
    });
    assert.ok(out.text.length <= 280);
    // Header + sender + subject should fit; snippet content (the long
    // repeated phrase) should not appear in entirety.
    assert.ok(!out.text.includes(longSnippet));
  });

  it('always retains the header line and sender line', () => {
    const out = renderFounderText({
      view: viewFixture({
        subject: 'X'.repeat(500),
        body_snippet: 'Y'.repeat(500)
      }),
      rank: { label: 'important', score: 0.9 }
    });
    assert.ok(out.text.includes('FOMO · IMPORTANT'));
    assert.ok(out.text.includes('s***@school.edu'));
  });

  it('individually clips long subject + snippet fields with ellipsis', () => {
    const out = renderFounderText({
      view: viewFixture({
        subject: 'A'.repeat(200),
        body_snippet: 'B'.repeat(300)
      }),
      rank: { label: 'important', score: 0.9 }
    });
    // The full 200/300-length runs should NOT be present.
    assert.ok(!out.text.includes('A'.repeat(200)));
    assert.ok(!out.text.includes('B'.repeat(300)));
  });
});

describe('renderFounderText — whitespace + multi-line collapse', () => {
  it('collapses repeated whitespace inside subject', () => {
    const out = renderFounderText({
      view: viewFixture({ subject: 'Hello\n\n\n  World\t\tfriend' }),
      rank: { label: 'important', score: 0.9 }
    });
    assert.ok(out.text.includes('Hello World friend'));
  });

  it('collapses repeated whitespace inside snippet', () => {
    const out = renderFounderText({
      view: viewFixture({ body_snippet: 'Multi\n\nline\n\nsnippet' }),
      rank: { label: 'important', score: 0.9 }
    });
    assert.ok(out.text.includes('Multi line snippet'));
  });
});

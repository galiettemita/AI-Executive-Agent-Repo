import assert from 'node:assert/strict';
import { describe, it } from 'node:test';

import {
  DEFAULT_EGRESS_OPTIONS,
  applyEgressForRanker,
  applyEgressForReplyParser,
  applyEgressForSlackCard,
  maskSenderEmail,
  type RawEmailContext
} from './egress-policy.ts';

function makeRaw(overrides: Partial<RawEmailContext> = {}): RawEmailContext {
  return {
    message_id: 'msg_abc',
    thread_id: 'thr_xyz',
    sender_email: 'sarah.j@school.edu',
    sender_name: 'Sarah Johnson',
    subject: 'Interview form due tonight',
    body_plain: 'Hi Albert, please remember to submit the interview form by 9pm tonight. Thanks, Sarah',
    body_html: '<html><body><b>Hi Albert,</b> please remember to submit...</body></html>',
    headers: {
      'Authentication-Results': 'spf=pass',
      'Received': 'from school.edu by gmail.com',
      'X-Internal-Tag': 'highly-sensitive-do-not-leak'
    },
    attachments: [{ filename: 'form.pdf', size_bytes: 12345 }],
    received_at: new Date('2026-05-22T18:30:00.000Z'),
    ...overrides
  };
}

describe('maskSenderEmail', () => {
  it('masks local part but preserves domain', () => {
    assert.equal(maskSenderEmail('sarah.j@school.edu'), 's***@school.edu');
    assert.equal(maskSenderEmail('a@b.co'), 'a***@b.co');
  });

  it('returns <masked> for malformed addresses', () => {
    assert.equal(maskSenderEmail(''), '<masked>');
    assert.equal(maskSenderEmail('no-at-sign'), '<masked>');
    assert.equal(maskSenderEmail('@nolocal.com'), '<masked>');
    assert.equal(maskSenderEmail('trailing@'), '<masked>');
  });
});

describe('applyEgressForRanker', () => {
  it('returns sender, subject, snippet, and structural metadata', () => {
    const view = applyEgressForRanker(makeRaw());
    assert.equal(view.purpose, 'model_ranker');
    assert.equal(view.sender_email, 'sarah.j@school.edu');
    assert.equal(view.sender_name, 'Sarah Johnson');
    assert.equal(view.subject, 'Interview form due tonight');
    assert.ok(view.body_snippet.length > 0);
    assert.equal(view.received_at, '2026-05-22T18:30:00.000Z');
    assert.equal(view.has_attachments, true);
    assert.equal(view.attachment_count, 1);
    assert.equal(view.message_id, 'msg_abc');
    assert.equal(view.thread_id, 'thr_xyz');
  });

  it('never includes body_html or headers', () => {
    const view = applyEgressForRanker(makeRaw());
    const serialized = JSON.stringify(view);
    assert.equal(serialized.includes('<html>'), false, 'HTML body leaked into view');
    assert.equal(serialized.includes('<b>'), false, 'HTML tag leaked into view');
    assert.equal(serialized.includes('Authentication-Results'), false, 'header leaked');
    assert.equal(serialized.includes('X-Internal-Tag'), false, 'header leaked');
    assert.equal(serialized.includes('highly-sensitive-do-not-leak'), false, 'header value leaked');
  });

  it('never includes attachment filenames (only count)', () => {
    const view = applyEgressForRanker(makeRaw());
    const serialized = JSON.stringify(view);
    assert.equal(serialized.includes('form.pdf'), false, 'attachment filename leaked');
  });

  it('truncates the body snippet to ranker_snippet_max_chars', () => {
    const longBody = 'a'.repeat(5000);
    const view = applyEgressForRanker(makeRaw({ body_plain: longBody }), {
      ...DEFAULT_EGRESS_OPTIONS,
      ranker_snippet_max_chars: 50
    });
    // 50 chars from longBody (all 'a'), trimEnd is a no-op for 'a's, plus ellipsis
    assert.ok(view.body_snippet.length <= 51, `snippet too long: ${view.body_snippet.length}`);
    assert.ok(view.body_snippet.endsWith('…'), 'truncated snippet must end with ellipsis');
  });

  it('does not append ellipsis when body fits within max chars', () => {
    const view = applyEgressForRanker(makeRaw({ body_plain: 'short' }));
    assert.equal(view.body_snippet, 'short');
    assert.equal(view.body_snippet.endsWith('…'), false);
  });

  it('collapses newlines and whitespace in the snippet', () => {
    const view = applyEgressForRanker(makeRaw({ body_plain: 'line1\n\n\tline2  \n   line3' }));
    assert.equal(view.body_snippet, 'line1 line2 line3');
  });

  it('handles missing attachments as zero count', () => {
    const view = applyEgressForRanker(makeRaw({ attachments: undefined }));
    assert.equal(view.has_attachments, false);
    assert.equal(view.attachment_count, 0);
  });

  it('returned view is frozen', () => {
    const view = applyEgressForRanker(makeRaw());
    assert.throws(() => {
      (view as unknown as { subject: string }).subject = 'mutated';
    });
  });
});

describe('applyEgressForSlackCard', () => {
  it('masks sender email and uses tighter snippet limit', () => {
    const view = applyEgressForSlackCard(makeRaw());
    assert.equal(view.purpose, 'slack_founder_card');
    assert.equal(view.sender_email_masked, 's***@school.edu');
    assert.equal(view.subject, 'Interview form due tonight');
    assert.ok(view.body_snippet.length > 0);
    assert.equal(view.message_id, 'msg_abc');
  });

  it('never includes the raw sender email anywhere in the view', () => {
    const view = applyEgressForSlackCard(makeRaw());
    const serialized = JSON.stringify(view);
    assert.equal(serialized.includes('sarah.j@'), false, 'raw sender local part leaked into Slack view');
  });

  it('never includes body_html or headers', () => {
    const view = applyEgressForSlackCard(makeRaw());
    const serialized = JSON.stringify(view);
    assert.equal(serialized.includes('<html>'), false);
    assert.equal(serialized.includes('X-Internal-Tag'), false);
    assert.equal(serialized.includes('highly-sensitive-do-not-leak'), false);
  });

  it('Slack snippet max defaults tighter than ranker snippet max', () => {
    assert.ok(
      DEFAULT_EGRESS_OPTIONS.slack_snippet_max_chars < DEFAULT_EGRESS_OPTIONS.ranker_snippet_max_chars,
      'Slack snippet must default tighter than ranker (friend privacy)'
    );
  });

  it('truncates body snippet to slack_snippet_max_chars when long', () => {
    const longBody = 'x'.repeat(1000);
    const view = applyEgressForSlackCard(makeRaw({ body_plain: longBody }), {
      ...DEFAULT_EGRESS_OPTIONS,
      slack_snippet_max_chars: 30
    });
    assert.ok(view.body_snippet.length <= 31);
    assert.ok(view.body_snippet.endsWith('…'));
  });

  it('returned view is frozen', () => {
    const view = applyEgressForSlackCard(makeRaw());
    assert.throws(() => {
      (view as unknown as { subject: string }).subject = 'mutated';
    });
  });
});

describe('applyEgressForReplyParser', () => {
  it('includes user reply text + minimal alert context', () => {
    const view = applyEgressForReplyParser('remind me later tonight', {
      subject: 'Interview form due tonight',
      sender_name: 'Sarah Johnson',
      message_id: 'msg_abc'
    });
    assert.equal(view.purpose, 'model_reply_parser');
    assert.equal(view.user_reply_text, 'remind me later tonight');
    assert.equal(view.alert_subject, 'Interview form due tonight');
    assert.equal(view.alert_sender_name, 'Sarah Johnson');
    assert.equal(view.alert_message_id, 'msg_abc');
  });

  it('NEVER carries the email body — only user reply text + alert metadata', () => {
    const view = applyEgressForReplyParser('ok', {
      subject: 'Reminder',
      message_id: 'm1'
    });
    const serialized = JSON.stringify(view);
    assert.equal('body_plain' in view, false);
    assert.equal('body_html' in view, false);
    assert.equal('body_snippet' in view, false);
    assert.equal(serialized.includes('body_plain'), false);
  });

  it('alert_sender_name is undefined when not provided', () => {
    const view = applyEgressForReplyParser('ok', { subject: 'X', message_id: 'm' });
    assert.equal(view.alert_sender_name, undefined);
  });

  it('returned view is frozen', () => {
    const view = applyEgressForReplyParser('ok', { subject: 'X', message_id: 'm' });
    assert.throws(() => {
      (view as unknown as { user_reply_text: string }).user_reply_text = 'mutated';
    });
  });
});

describe('DEFAULT_EGRESS_OPTIONS', () => {
  it('is frozen', () => {
    assert.throws(() => {
      (DEFAULT_EGRESS_OPTIONS as unknown as { ranker_snippet_max_chars: number }).ranker_snippet_max_chars = 9999;
    });
  });

  it('default ranker snippet is a reasonable bound (between 100 and 1000 chars)', () => {
    assert.ok(DEFAULT_EGRESS_OPTIONS.ranker_snippet_max_chars >= 100);
    assert.ok(DEFAULT_EGRESS_OPTIONS.ranker_snippet_max_chars <= 1000);
  });
});

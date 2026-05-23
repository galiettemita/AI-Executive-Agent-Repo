// Egress Policy — controls what data leaves Brevio for model calls, Slack
// cards, or any other downstream surface that is not the original Gmail
// boundary.
//
// FOMO_PLAN §9.4 and FOMO_DESIGN §17:
//   * no raw full Gmail body by default,
//   * ranker receives safe context (sender, subject, limited snippet),
//   * reply parser receives user text + limited alert context,
//   * no friend email body to Slack (mask sender; truncate snippet),
//   * no private email bodies in repo or logs.
//
// Phase 2C ships substrate only — pure functions that take raw context and
// return narrowed views. Nothing in v0.1 calls these yet; Phase 3 wires them
// in front of the ranker, reply parser, and Slack card writer.
//
// Safety properties tested:
//   * RawEmailContext.body_html and .headers NEVER appear in any view
//   * full body is never present — only a truncated snippet from body_plain
//   * snippet respects max char length
//   * Slack view masks the sender email (e.g. s***@school.edu)
//   * reply parser view excludes the email body entirely

export type EgressPurpose = 'model_ranker' | 'model_reply_parser' | 'slack_founder_card';

export interface RawEmailContext {
  // Stable identifier from Gmail.
  message_id: string;
  thread_id?: string;
  sender_email: string;
  sender_name?: string;
  subject: string;
  // Plaintext body. body_html is intentionally separate and never egressed.
  body_plain: string;
  body_html?: string;
  // Raw Gmail headers (Authentication-Results, Received, etc.) — never egressed.
  headers?: Record<string, string>;
  attachments?: readonly { filename: string; size_bytes: number }[];
  received_at: Date;
}

export interface RankerEgressView {
  readonly purpose: 'model_ranker';
  readonly sender_email: string;
  readonly sender_name: string | undefined;
  readonly subject: string;
  readonly body_snippet: string;
  readonly received_at: string;
  readonly has_attachments: boolean;
  readonly attachment_count: number;
  readonly message_id: string;
  readonly thread_id: string | undefined;
}

export interface SlackEgressView {
  readonly purpose: 'slack_founder_card';
  // Sender masking: 's***@school.edu' (first char + *** + @domain). Domain is
  // preserved because founders need to see *which kind* of sender they're
  // approving (school vs. work vs. social), but the local part is hidden so a
  // friend's specific address never appears in Slack history.
  readonly sender_email_masked: string;
  readonly sender_name: string | undefined;
  readonly subject: string;
  readonly body_snippet: string;
  readonly received_at: string;
  readonly message_id: string;
}

export interface ReplyParserEgressView {
  readonly purpose: 'model_reply_parser';
  readonly user_reply_text: string;
  // Minimal alert context — enough for the parser to disambiguate sender
  // references like "ignore from sarah", but never the email body.
  readonly alert_subject: string;
  readonly alert_sender_name: string | undefined;
  readonly alert_message_id: string;
}

export interface EgressOptions {
  readonly ranker_snippet_max_chars: number;
  readonly slack_snippet_max_chars: number;
}

export const DEFAULT_EGRESS_OPTIONS: EgressOptions = Object.freeze({
  // ~one tweet — enough signal for ranking, far short of leaking the message.
  ranker_snippet_max_chars: 280,
  // Tighter for Slack so the founder sees a preview, not the contents.
  slack_snippet_max_chars: 200
});

// Normalize whitespace + truncate. Strips newlines, tabs, and consecutive
// spaces. Appends a single ellipsis character when truncation occurred.
function makeSnippet(bodyPlain: string, maxChars: number): string {
  const collapsed = bodyPlain.replace(/\s+/g, ' ').trim();
  if (collapsed.length <= maxChars) return collapsed;
  return collapsed.slice(0, maxChars).trimEnd() + '…';
}

// 'sarah.j@school.edu' → 's***@school.edu'.
// Empty / malformed addresses fall through to a generic mask so a bad input
// never leaks the raw string.
export function maskSenderEmail(email: string): string {
  const at = email.indexOf('@');
  if (at <= 0 || at === email.length - 1) return '<masked>';
  const local = email.slice(0, at);
  const domain = email.slice(at + 1);
  const firstChar = local.charAt(0);
  return `${firstChar}***@${domain}`;
}

export function applyEgressForRanker(
  raw: RawEmailContext,
  opts: EgressOptions = DEFAULT_EGRESS_OPTIONS
): RankerEgressView {
  return Object.freeze({
    purpose: 'model_ranker',
    sender_email: raw.sender_email,
    sender_name: raw.sender_name,
    subject: raw.subject,
    body_snippet: makeSnippet(raw.body_plain, opts.ranker_snippet_max_chars),
    received_at: raw.received_at.toISOString(),
    has_attachments: (raw.attachments?.length ?? 0) > 0,
    attachment_count: raw.attachments?.length ?? 0,
    message_id: raw.message_id,
    thread_id: raw.thread_id
  });
}

export function applyEgressForSlackCard(
  raw: RawEmailContext,
  opts: EgressOptions = DEFAULT_EGRESS_OPTIONS
): SlackEgressView {
  return Object.freeze({
    purpose: 'slack_founder_card',
    sender_email_masked: maskSenderEmail(raw.sender_email),
    sender_name: raw.sender_name,
    subject: raw.subject,
    body_snippet: makeSnippet(raw.body_plain, opts.slack_snippet_max_chars),
    received_at: raw.received_at.toISOString(),
    message_id: raw.message_id
  });
}

export function applyEgressForReplyParser(
  userReplyText: string,
  alertContext: { subject: string; sender_name?: string; message_id: string }
): ReplyParserEgressView {
  return Object.freeze({
    purpose: 'model_reply_parser',
    user_reply_text: userReplyText,
    alert_subject: alertContext.subject,
    alert_sender_name: alertContext.sender_name,
    alert_message_id: alertContext.message_id
  });
}

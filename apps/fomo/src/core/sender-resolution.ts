// Sender resolution — Phase v0.5.7 (Modified Q2.B).
//
// Implements the founder-locked sender display chain for the Human
// Message Renderer (apps/fomo/src/core/human-message-renderer.ts). Given
// the safe egress-projected sender_name + raw email parts, returns the
// "display token" that goes into the HMR sentence opener AND the
// resolution_path enum value that gets persisted in audit detail.
//
// 3E.1 PRESERVED: pure deterministic functions. No I/O, no LLM, no
// clock. Same constraints as the renderer.
//
// Founder rules (locked 2026-06-06, see memory project_v05-7-scope
// "Modified Q2.B" + memory feedback_brevio-voice-rules):
//   1. first_token(sender_name) if sender_name is safe + human-looking
//   2. domain-derived label for obvious system / no-reply / notification
//      senders. Detection: (a) local-part regex matches known system
//      tokens (no-reply, notifications, support, mailer-daemon, ...) OR
//      (b) sender domain matches the curated SYSTEM_SENDER_DOMAINS table.
//   3. email-local-part-derived first name ONLY IF the local part is
//      clearly human-readable (john.doe / jane_smith pattern, both
//      tokens ≥3 letters, pure ASCII letters).
//   4. otherwise → "Someone".
//
// Anti-rules (founder corrections):
//   * Do NOT produce awkward names like "galiettemita" → "Galiettemita".
//     The local-part path is GATED on the human-readable pattern.
//   * Do NOT expose masked email (e.g. "g***@icloud.com") as the
//     display token. There is no masked-email fallback in this chain.
//   * Pronouns ("she" / "he" / "they") are an LLM voice concern, NOT a
//     renderer concern. The renderer only chooses a display token; the
//     ranker-v0.2.0 prompt handles pronoun selection in `rank.reason`
//     per feedback_brevio-voice-rules.

export type SenderResolutionPath = 'first_name' | 'domain_label' | 'email_local' | 'generic';

export interface SenderResolutionInput {
  // Raw From-header display name as projected by egress. May be empty.
  readonly sender_name: string | undefined;
  // The full raw email address. Used internally by this module only to
  // extract the local-part for pattern-matching and the domain root for
  // labelling. The display TOKEN this module returns NEVER includes the
  // raw email; the masked email is also forbidden from the display per
  // founder lock 2026-06-06.
  readonly sender_email: string;
}

export interface SenderResolutionOutput {
  // The display token that goes into the HMR sentence opener:
  //   "Galiette" / "GitHub" / "John" / "Someone"
  readonly display: string;
  // The path the chain took. Persisted to audit detail
  // (sender_resolution_path) per Q6.A. Structural enum only — never
  // carries user content. Smoke-evidence C7 reads the distribution.
  readonly path: SenderResolutionPath;
}

/* -------------------------------------------------------------------- */
/* Curated SYSTEM_SENDER_DOMAINS table                                  */
/* -------------------------------------------------------------------- */
//
// Maps a known SaaS / service domain to the friendly display label the
// user is likely to recognize. Extensible by adding entries here — no
// scope-creep into a plugin registry per Q6.A "restraint" lock.
//
// Domain match is on the REGISTRABLE root (e.g. "github.com" matches
// any `*.github.com` sender). Subdomain noise (no-reply.notifications.
// github.com) collapses to the canonical "GitHub" label so the user
// sees a consistent opener across the service's sub-products.
//
// Founder principle: this list curates the top SaaS senders a typical
// founder receives. It is NOT exhaustive; uncurated domains fall
// through to capitalized-domain-root (see deriveDomainLabel).
const SYSTEM_SENDER_DOMAINS: ReadonlyMap<string, string> = new Map([
  ['github.com', 'GitHub'],
  ['stripe.com', 'Stripe'],
  ['linear.app', 'Linear'],
  ['notion.so', 'Notion'],
  ['slack.com', 'Slack'],
  ['figma.com', 'Figma'],
  ['vercel.com', 'Vercel'],
  ['netlify.com', 'Netlify'],
  ['cloudflare.com', 'Cloudflare'],
  ['aws.amazon.com', 'AWS'],
  ['amazonaws.com', 'AWS'],
  ['google.com', 'Google'],
  ['accounts.google.com', 'Google'],
  ['openai.com', 'OpenAI'],
  ['anthropic.com', 'Anthropic'],
  ['linkedin.com', 'LinkedIn'],
  ['twitter.com', 'Twitter'],
  ['x.com', 'X'],
  ['youtube.com', 'YouTube'],
  ['discord.com', 'Discord'],
  ['mailgun.org', 'Mailgun'],
  ['sendgrid.net', 'SendGrid'],
  ['mailchimp.com', 'Mailchimp'],
  ['intercom.com', 'Intercom'],
  ['intercom.io', 'Intercom'],
  ['zoom.us', 'Zoom'],
  ['dropbox.com', 'Dropbox'],
  ['microsoft.com', 'Microsoft'],
  ['apple.com', 'Apple'],
  ['icloud.com', 'iCloud'],
  ['paypal.com', 'PayPal'],
  ['venmo.com', 'Venmo'],
  ['airbnb.com', 'Airbnb'],
  ['uber.com', 'Uber'],
  ['lyft.com', 'Lyft'],
  ['doordash.com', 'DoorDash'],
  ['instacart.com', 'Instacart'],
  ['shopify.com', 'Shopify']
]);

// Local-part tokens that strongly indicate a system/no-reply sender.
// Matched case-insensitively against the leading token of the local part
// (i.e. before any "+", "." or "_" separator).
const SYSTEM_LOCAL_PART_REGEX =
  /^(no-?reply|donotreply|noreply|notifications?|alerts?|support|hello|info|team|hi|mailer-daemon|postmaster|admin|webmaster|system|automated|bounce|bounces)([.+_-]|$)/i;

// First-token noise words that look human but are actually From-header
// artifacts. Reject these so we don't end up with "Via" as a display.
const FIRST_NAME_NOISE_BLOCKLIST: ReadonlySet<string> = new Set([
  'via',
  'on',
  'and',
  'the',
  'from',
  'sent',
  'mailer',
  'noreply',
  'no-reply',
  'donotreply',
  'notifications',
  'notification',
  'alerts',
  'alert',
  'support',
  'team',
  'admin',
  'info',
  'hello',
  'hi',
  'system',
  'automated'
]);

/* -------------------------------------------------------------------- */
/* Public helpers                                                       */
/* -------------------------------------------------------------------- */

// Splits an email into [localPart, domain]. Returns null on malformed
// input. Domain is lowercased. Local part keeps original casing because
// the human-readable pattern test (john.doe etc.) is case-insensitive
// anyway.
export function splitEmail(email: string): { local: string; domain: string } | null {
  const at = email.indexOf('@');
  if (at <= 0 || at === email.length - 1) return null;
  const local = email.slice(0, at);
  const domain = email.slice(at + 1).toLowerCase().trim();
  if (!local || !domain || domain.indexOf('.') === -1) return null;
  return { local, domain };
}

// Returns true if the sender's local part OR domain matches the system-
// sender heuristics. Used by Q2.B step 2.
export function isSystemSender(email: string): boolean {
  const parts = splitEmail(email);
  if (!parts) return false;
  if (SYSTEM_LOCAL_PART_REGEX.test(parts.local)) return true;
  // Domain match — exact or any registered subdomain.
  for (const knownDomain of SYSTEM_SENDER_DOMAINS.keys()) {
    if (parts.domain === knownDomain || parts.domain.endsWith('.' + knownDomain)) {
      return true;
    }
  }
  return false;
}

// Derives a display label from the sender's domain. Uses the curated
// SYSTEM_SENDER_DOMAINS table when matched; otherwise capitalizes the
// registrable root (e.g. "acme.com" → "Acme"). Per founder lock, NEVER
// returns multi-word slugs with awkward casing ("Unknown-saas") —
// hyphens in the root get stripped to the leading token.
export function deriveDomainLabel(email: string): string | null {
  const parts = splitEmail(email);
  if (!parts) return null;
  // Curated table — match exact or known parent.
  for (const [knownDomain, label] of SYSTEM_SENDER_DOMAINS.entries()) {
    if (parts.domain === knownDomain || parts.domain.endsWith('.' + knownDomain)) {
      return label;
    }
  }
  // Uncurated — capitalize the leading token of the registrable root.
  // "acme.com" → "Acme". "marketing.acme.com" → registrable is "acme.com"
  // (we approximate by taking the second-to-last label since we don't
  // ship a TLD list). For "mail.foo.co.uk" we'd produce "Foo" which is
  // close enough for v0.5.7 — the smoke surfaces edge cases as §11 candidates.
  const labels = parts.domain.split('.');
  if (labels.length < 2) return null;
  // Take the second-to-last label, then strip anything after the first
  // non-letter char (handles "acme-corp.com" → "Acme").
  const root = labels[labels.length - 2] ?? '';
  const leading = root.split(/[^a-z0-9]/i)[0] ?? '';
  if (!leading || leading.length < 2) return null;
  return leading.charAt(0).toUpperCase() + leading.slice(1).toLowerCase();
}

// Extracts a safe first-name token from the sender's display name.
// Returns null if no safe token is found.
//
// Safe rules (founder lock 2026-06-06):
//   * First whitespace-separated token only (drops "Mita" from
//     "Galiette Mita"; drops "via LinkedIn" from "Galiette Mita via
//     LinkedIn")
//   * Pure-letter token with optional internal hyphen / apostrophe
//     (allows "Jean-Paul", "O'Brien", "Sarah"). Rejects digits, "@",
//     special chars.
//   * ≥ 2 chars (allows "Ed", "Jo").
//   * NOT in FIRST_NAME_NOISE_BLOCKLIST ("via", "team", etc.).
//   * Capitalizes the first letter so "galiette" (lowercase header) →
//     "Galiette".
export function extractFirstNameFromSenderName(senderName: string | undefined): string | null {
  if (!senderName) return null;
  const trimmed = senderName.trim();
  if (!trimmed) return null;
  const firstToken = trimmed.split(/\s+/)[0] ?? '';
  if (!firstToken) return null;
  // Pure alphabetic + hyphen + apostrophe, ≥ 2 chars.
  if (!/^[A-Za-z][A-Za-z'-]+$/.test(firstToken)) return null;
  if (firstToken.length < 2) return null;
  const lower = firstToken.toLowerCase();
  if (FIRST_NAME_NOISE_BLOCKLIST.has(lower)) return null;
  return firstToken.charAt(0).toUpperCase() + firstToken.slice(1).toLowerCase();
}

// Extracts a safe first-name token from the email LOCAL part when it
// follows a clearly human-readable pattern.
//
// Patterns accepted:
//   * "john.doe" → "John"
//   * "jane_smith" → "Jane"
//   * "alex-jones" → "Alex"
//   Both tokens must be ≥ 3 chars, pure ASCII letters, and not match
//   the FIRST_NAME_NOISE_BLOCKLIST.
//
// Rejected (returns null):
//   * "galiettemita" — single token (no separator). Per founder lock,
//     DO NOT produce "Galiettemita". Falls through to "Someone".
//   * "j.d" — both tokens < 3 chars
//   * "john123" — contains digits
//   * "noreply.bounces" — first token in blocklist
//
// Pure function. No I/O.
export function extractFirstNameFromEmailLocal(email: string): string | null {
  const parts = splitEmail(email);
  if (!parts) return null;
  const local = parts.local;
  // Strip any "+tag" suffix used for filtering (e.g. "john.doe+filter").
  const beforePlus = local.split('+')[0] ?? local;
  // Patterns: separated by '.', '_', or '-'.
  const m = beforePlus.match(/^([a-z]{3,})[._-]([a-z]{3,})$/i);
  if (!m) return null;
  const [, firstRaw, lastRaw] = m;
  if (!firstRaw || !lastRaw) return null;
  const firstLower = firstRaw.toLowerCase();
  const lastLower = lastRaw.toLowerCase();
  if (FIRST_NAME_NOISE_BLOCKLIST.has(firstLower)) return null;
  if (FIRST_NAME_NOISE_BLOCKLIST.has(lastLower)) return null;
  return firstRaw.charAt(0).toUpperCase() + firstRaw.slice(1).toLowerCase();
}

/* -------------------------------------------------------------------- */
/* Q2.B chain — the canonical sender resolution                         */
/* -------------------------------------------------------------------- */

// Generic display token used by Q2.B step 4 (no safer option). The
// renderer wraps this in audit field sender_resolution_path='generic'.
export const SENDER_RESOLUTION_GENERIC_DISPLAY = 'Someone';

// Runs the Modified Q2.B chain in order. Returns the first match.
//
// Order:
//   1. first_token(sender_name) if safe and human-looking → 'first_name'
//   2. domain-derived label if isSystemSender → 'domain_label'
//   3. email-local-derived first name if pattern matches → 'email_local'
//   4. 'Someone' → 'generic'
//
// Founder corrections (locked 2026-06-06):
//   * NO awkward Galiettemita-style names — gated at step 3.
//   * NO masked-email in opener — no such fallback in this chain.
export function resolveSender(input: SenderResolutionInput): SenderResolutionOutput {
  // Step 1: first-name from sender_name.
  const firstNameFromHeader = extractFirstNameFromSenderName(input.sender_name);
  if (firstNameFromHeader) {
    return Object.freeze({ display: firstNameFromHeader, path: 'first_name' });
  }
  // Step 2: domain-label for system senders.
  if (isSystemSender(input.sender_email)) {
    const label = deriveDomainLabel(input.sender_email);
    if (label) {
      return Object.freeze({ display: label, path: 'domain_label' });
    }
  }
  // Step 3: email-local human-readable pattern.
  const firstNameFromLocal = extractFirstNameFromEmailLocal(input.sender_email);
  if (firstNameFromLocal) {
    return Object.freeze({ display: firstNameFromLocal, path: 'email_local' });
  }
  // Step 4: generic.
  return Object.freeze({
    display: SENDER_RESOLUTION_GENERIC_DISPLAY,
    path: 'generic'
  });
}

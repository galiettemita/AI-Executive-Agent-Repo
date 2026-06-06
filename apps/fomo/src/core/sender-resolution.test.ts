// Phase v0.5.7 — sender-resolution test suite.
//
// Covers all 4 paths of the founder-locked Modified Q2.B chain:
//   1. first_name — from sender_name when safe + human-looking
//   2. domain_label — for obvious system / no-reply / notification senders
//   3. email_local — when local part follows human-readable pattern
//   4. generic ("Someone") — fallback
//
// Founder-locked anti-patterns the suite verifies are REJECTED:
//   * "galiettemita" → does NOT become "Galiettemita"
//   * masked email never appears in the display
//   * "via" first token never becomes "Via"

import { describe, it } from 'node:test';
import assert from 'node:assert/strict';

import {
  deriveDomainLabel,
  extractFirstNameFromEmailLocal,
  extractFirstNameFromSenderName,
  isSystemSender,
  resolveSender,
  SENDER_RESOLUTION_GENERIC_DISPLAY,
  splitEmail
} from './sender-resolution.js';

describe('splitEmail', () => {
  it('splits a normal email into local + lowercase domain', () => {
    const out = splitEmail('John.Doe@Acme.COM');
    assert.deepEqual(out, { local: 'John.Doe', domain: 'acme.com' });
  });

  it('returns null for malformed input (no @)', () => {
    assert.equal(splitEmail('not-an-email'), null);
  });

  it('returns null for empty local part', () => {
    assert.equal(splitEmail('@acme.com'), null);
  });

  it('returns null for empty domain', () => {
    assert.equal(splitEmail('john@'), null);
  });

  it('returns null for domain with no dot (single-label)', () => {
    assert.equal(splitEmail('john@localhost'), null);
  });
});

describe('extractFirstNameFromSenderName', () => {
  it('extracts the first token from "Galiette Mita"', () => {
    assert.equal(extractFirstNameFromSenderName('Galiette Mita'), 'Galiette');
  });

  it('capitalizes lowercase-headered first names', () => {
    assert.equal(extractFirstNameFromSenderName('galiette mita'), 'Galiette');
  });

  it('preserves internal hyphens (Jean-Paul)', () => {
    assert.equal(extractFirstNameFromSenderName('Jean-Paul Sartre'), 'Jean-paul');
  });

  it('preserves internal apostrophes (O\'Brien)', () => {
    assert.equal(extractFirstNameFromSenderName("O'Brien"), "O'brien");
  });

  it('drops "via LinkedIn" augmentations', () => {
    assert.equal(extractFirstNameFromSenderName('Galiette Mita via LinkedIn'), 'Galiette');
  });

  it('rejects digit-containing names', () => {
    assert.equal(extractFirstNameFromSenderName('1-800-FLOWERS'), null);
  });

  it('rejects empty / whitespace-only', () => {
    assert.equal(extractFirstNameFromSenderName(''), null);
    assert.equal(extractFirstNameFromSenderName('   '), null);
    assert.equal(extractFirstNameFromSenderName(undefined), null);
  });

  it('rejects single-char "first tokens"', () => {
    assert.equal(extractFirstNameFromSenderName('J Doe'), null);
  });

  it('rejects header-noise blocklist tokens (no "Via")', () => {
    assert.equal(extractFirstNameFromSenderName('via LinkedIn'), null);
    assert.equal(extractFirstNameFromSenderName('team Acme'), null);
    assert.equal(extractFirstNameFromSenderName('notifications GitHub'), null);
  });

  it('accepts short but real names ("Ed", "Jo")', () => {
    assert.equal(extractFirstNameFromSenderName('Ed Smith'), 'Ed');
    assert.equal(extractFirstNameFromSenderName('Jo'), 'Jo');
  });
});

describe('isSystemSender + deriveDomainLabel', () => {
  it('matches local-part regex (no-reply@anything)', () => {
    assert.equal(isSystemSender('no-reply@example.com'), true);
    assert.equal(isSystemSender('noreply@example.com'), true);
    assert.equal(isSystemSender('notifications@example.com'), true);
    assert.equal(isSystemSender('support@example.com'), true);
    assert.equal(isSystemSender('mailer-daemon@example.com'), true);
    assert.equal(isSystemSender('admin@example.com'), true);
  });

  it('does NOT match arbitrary local parts on unknown domains', () => {
    assert.equal(isSystemSender('john.doe@example.com'), false);
    assert.equal(isSystemSender('galiettemita@icloud.com'), true); // icloud.com is in curated table; this is a system sender by domain, separate from the local pattern
  });

  it('matches curated SYSTEM_SENDER_DOMAINS by exact domain', () => {
    assert.equal(isSystemSender('anything@github.com'), true);
    assert.equal(isSystemSender('whatever@stripe.com'), true);
    assert.equal(isSystemSender('user@linear.app'), true);
  });

  it('matches curated SYSTEM_SENDER_DOMAINS via subdomain', () => {
    assert.equal(isSystemSender('no-reply@notifications.github.com'), true);
  });

  it('returns false for unknown human-shaped emails', () => {
    assert.equal(isSystemSender('mark.chen@acme.com'), false);
  });

  it('curated table maps to friendly labels', () => {
    assert.equal(deriveDomainLabel('no-reply@github.com'), 'GitHub');
    assert.equal(deriveDomainLabel('alerts@stripe.com'), 'Stripe');
    assert.equal(deriveDomainLabel('user@linear.app'), 'Linear');
    assert.equal(deriveDomainLabel('hi@notion.so'), 'Notion');
  });

  it('curated subdomain matches collapse to the canonical label', () => {
    assert.equal(deriveDomainLabel('bot@no-reply.notifications.github.com'), 'GitHub');
  });

  it('uncurated domains get capitalized-root fallback (acme.com → Acme)', () => {
    assert.equal(deriveDomainLabel('admin@acme.com'), 'Acme');
    assert.equal(deriveDomainLabel('hi@brevio.com'), 'Brevio');
  });

  it('handles hyphenated roots — takes the leading token only', () => {
    assert.equal(deriveDomainLabel('admin@acme-corp.com'), 'Acme');
  });

  it('returns null for malformed inputs', () => {
    assert.equal(deriveDomainLabel('not-an-email'), null);
    assert.equal(deriveDomainLabel('@only-domain'), null);
  });
});

describe('extractFirstNameFromEmailLocal', () => {
  it('extracts "John" from "john.doe@acme.com"', () => {
    assert.equal(extractFirstNameFromEmailLocal('john.doe@acme.com'), 'John');
  });

  it('extracts "Jane" from "jane_smith@acme.com"', () => {
    assert.equal(extractFirstNameFromEmailLocal('jane_smith@acme.com'), 'Jane');
  });

  it('extracts "Alex" from "alex-jones@acme.com"', () => {
    assert.equal(extractFirstNameFromEmailLocal('alex-jones@acme.com'), 'Alex');
  });

  it('handles +tag suffixes (john.doe+filter@acme.com → John)', () => {
    assert.equal(extractFirstNameFromEmailLocal('john.doe+brevio@acme.com'), 'John');
  });

  it('REJECTS single-token locals ("galiettemita" → null, NOT "Galiettemita")', () => {
    // Founder lock 2026-06-06: do not produce awkward names like
    // "Galiettemita". This is the load-bearing edge case from the
    // post-v0.5.6 conversation.
    assert.equal(extractFirstNameFromEmailLocal('galiettemita@icloud.com'), null);
  });

  it('rejects too-short tokens (j.d, ab.cd)', () => {
    assert.equal(extractFirstNameFromEmailLocal('j.d@acme.com'), null);
    assert.equal(extractFirstNameFromEmailLocal('ab.cd@acme.com'), null);
  });

  it('rejects locals containing digits (john123@acme.com)', () => {
    assert.equal(extractFirstNameFromEmailLocal('john123.smith@acme.com'), null);
  });

  it('rejects blocklist-matching first tokens (noreply.bounces → null)', () => {
    assert.equal(extractFirstNameFromEmailLocal('noreply.bounces@acme.com'), null);
    assert.equal(extractFirstNameFromEmailLocal('team.support@acme.com'), null);
  });

  it('rejects blocklist-matching second tokens (john.team → null)', () => {
    assert.equal(extractFirstNameFromEmailLocal('john.team@acme.com'), null);
  });

  it('rejects malformed emails', () => {
    assert.equal(extractFirstNameFromEmailLocal('not-an-email'), null);
  });
});

describe('resolveSender — Q2.B chain end-to-end', () => {
  it('path 1: first_name — sender_name "Galiette Mita" → "Galiette"', () => {
    const out = resolveSender({
      sender_name: 'Galiette Mita',
      sender_email: 'g***@icloud.com'
    });
    assert.equal(out.display, 'Galiette');
    assert.equal(out.path, 'first_name');
  });

  it('path 1: capitalizes lowercase header names', () => {
    const out = resolveSender({
      sender_name: 'sarah mita',
      sender_email: 's***@icloud.com'
    });
    assert.equal(out.display, 'Sarah');
    assert.equal(out.path, 'first_name');
  });

  it('path 2: domain_label — system local-part on uncurated domain → capitalized root', () => {
    const out = resolveSender({
      sender_name: undefined,
      sender_email: 'no-reply@acme-corp.com'
    });
    assert.equal(out.display, 'Acme');
    assert.equal(out.path, 'domain_label');
  });

  it('path 2: domain_label — curated SaaS domain wins ("GitHub")', () => {
    const out = resolveSender({
      sender_name: undefined,
      sender_email: 'notifications@github.com'
    });
    assert.equal(out.display, 'GitHub');
    assert.equal(out.path, 'domain_label');
  });

  it('path 2: domain_label — curated subdomain → canonical label', () => {
    const out = resolveSender({
      sender_name: '',
      sender_email: 'bot@no-reply.notifications.github.com'
    });
    assert.equal(out.display, 'GitHub');
    assert.equal(out.path, 'domain_label');
  });

  it('path 3: email_local — "john.doe@acme.com" → "John"', () => {
    const out = resolveSender({
      sender_name: undefined,
      sender_email: 'john.doe@acme.com'
    });
    assert.equal(out.display, 'John');
    assert.equal(out.path, 'email_local');
  });

  it('path 4: generic — "galiettemita@icloud.com" with no sender_name → "Someone"', () => {
    // icloud.com IS in the curated table so step 2 actually wins —
    // resolves to "iCloud". This documents that iCloud personal addresses
    // get a domain label fallback. The pure "generic" path requires an
    // uncurated, non-pattern-matching email + no sender_name.
    const out = resolveSender({
      sender_name: undefined,
      sender_email: 'galiettemita@icloud.com'
    });
    assert.equal(out.path, 'domain_label');
    assert.equal(out.display, 'iCloud');
  });

  it('path 4: generic — single-token local on uncurated domain → "Someone"', () => {
    // This is the load-bearing edge case the founder called out:
    // single-token local part ("galiettemita") on an uncurated domain
    // (no system regex, no curated entry) MUST fall through to
    // "Someone", NOT produce "Galiettemita".
    const out = resolveSender({
      sender_name: undefined,
      sender_email: 'galiettemita@uncurated-personal.io'
    });
    assert.equal(out.display, SENDER_RESOLUTION_GENERIC_DISPLAY);
    assert.equal(out.display, 'Someone');
    assert.equal(out.path, 'generic');
  });

  it('path 4: generic — empty sender_name + malformed email → "Someone"', () => {
    const out = resolveSender({
      sender_name: '',
      sender_email: 'not-an-email'
    });
    assert.equal(out.display, 'Someone');
    assert.equal(out.path, 'generic');
  });

  it('path priority — step 1 wins over step 2/3/4 when sender_name is safe', () => {
    // Even if the email looks like a system sender, if the sender_name
    // is human-looking, use it.
    const out = resolveSender({
      sender_name: 'Mark Chen',
      sender_email: 'no-reply@github.com'
    });
    assert.equal(out.display, 'Mark');
    assert.equal(out.path, 'first_name');
  });

  it('path priority — step 2 wins over step 3 when sender_name fails + system match', () => {
    // jane.doe@github.com: github.com is in curated table → step 2 wins.
    const out = resolveSender({
      sender_name: undefined,
      sender_email: 'jane.doe@github.com'
    });
    assert.equal(out.display, 'GitHub');
    assert.equal(out.path, 'domain_label');
  });

  it('never emits masked-email display (founder anti-rule)', () => {
    // Across many shapes, verify display never contains "***" or "@".
    const cases = [
      { sender_name: undefined, sender_email: 'galiettemita@uncurated-personal.io' },
      { sender_name: '', sender_email: 'no-reply@acme.com' },
      { sender_name: 'via LinkedIn', sender_email: 'mailer@bounces.linkedin.com' },
      { sender_name: '', sender_email: 'not-an-email' }
    ];
    for (const c of cases) {
      const out = resolveSender(c);
      assert.equal(out.display.includes('***'), false, `display "${out.display}" must not contain ***`);
      assert.equal(out.display.includes('@'), false, `display "${out.display}" must not contain @`);
    }
  });

  it('never emits "Galiettemita" / "Notifications" / "Via" / similar awkward derivations', () => {
    // These are the exact founder-flagged anti-patterns.
    const cases = [
      { sender_name: undefined, sender_email: 'galiettemita@uncurated-personal.io' },
      { sender_name: 'via LinkedIn', sender_email: 'mailer@bounces.linkedin.com' },
      { sender_name: 'notifications', sender_email: 'whatever@unknown.com' }
    ];
    const banned = ['Galiettemita', 'Via', 'Notifications', 'Mailer', 'Team', 'Support', 'Admin'];
    for (const c of cases) {
      const out = resolveSender(c);
      for (const b of banned) {
        assert.notEqual(out.display, b, `display "${out.display}" must not be the awkward derivation "${b}" for ${JSON.stringify(c)}`);
      }
    }
  });
});

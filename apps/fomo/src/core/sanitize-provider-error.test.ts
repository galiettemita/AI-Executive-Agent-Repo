// Phase v0.5.15 — Sanitized Provider Error Reasons unit tests.
//
// What unit tests prove for this chokepoint:
//   - The locked SanitizedReason set is closed; no caller can introduce a
//     novel reason without changing the type.
//   - Known provider codes map to the deterministic safe rows.
//   - Unknown but well-shaped codes pass through with generic reason.
//   - Non-token-shaped raw codes are ignored entirely (no leak path).
//   - HTTP status classification is locked to the documented table.
//   - Network codes route to 'network_error' (transient).
//   - Raw provider messages are NEVER inspected for content; only their
//     presence elevates 'unknown_error' to 'provider_error'.
//   - Privacy canary: forbidden substrings never appear in the output,
//     regardless of input.

import assert from 'node:assert/strict';
import { describe, it } from 'node:test';

import {
  SANITIZED_REASONS,
  _KNOWN_PROVIDER_CODE_MAP_FOR_TESTS,
  _KNOWN_NETWORK_CODES_FOR_TESTS,
  sanitizeProviderError,
  type SanitizedReason
} from './sanitize-provider-error.ts';

describe('SanitizedReason — locked closed set (no surprise reasons)', () => {
  it('contains exactly the 10 locked reasons (no more, no less)', () => {
    assert.equal(SANITIZED_REASONS.length, 10);
    assert.deepEqual(
      [...SANITIZED_REASONS].sort(),
      [
        'auth_error',
        'invalid_argument',
        'network_error',
        'not_found',
        'provider_error',
        'rate_limited',
        'recipient_not_registered',
        'recipient_opted_out',
        'temporary_provider_error',
        'unknown_error'
      ]
    );
  });

  it('SANITIZED_REASONS is frozen', () => {
    assert.throws(() => {
      (SANITIZED_REASONS as unknown as string[]).push('arbitrary_new_reason');
    });
  });
});

describe('sanitizeProviderError — known provider code → mapped row', () => {
  // SendBlue
  it("'OPTED_OUT' → recipient_opted_out", () => {
    const r = sanitizeProviderError({ raw_provider_code: 'OPTED_OUT' });
    assert.equal(r.error_code, 'OPTED_OUT');
    assert.equal(r.error_reason, 'recipient_opted_out');
  });

  it("SendBlue 'SpamRule' (mixed case) → recipient_opted_out after normalization", () => {
    const r = sanitizeProviderError({ raw_provider_code: 'SpamRule' });
    // 'SpamRule' normalizes to 'SPAMRULE' (uppercase; hyphen → underscore,
    // no other char-class conversion). SPAMRULE IS in the locked map, so
    // it maps to RECIPIENT_OPTED_OUT which has error_code='OPTED_OUT'.
    assert.equal(r.error_code, 'OPTED_OUT');
    assert.equal(r.error_reason, 'recipient_opted_out');
  });

  it("SendBlue 'RATE_LIMITED' → rate_limited", () => {
    const r = sanitizeProviderError({ raw_provider_code: 'RATE_LIMITED' });
    assert.equal(r.error_code, 'RATE_LIMITED');
    assert.equal(r.error_reason, 'rate_limited');
  });

  it("SendBlue 'UNAUTHORIZED' → auth_error", () => {
    const r = sanitizeProviderError({ raw_provider_code: 'UNAUTHORIZED' });
    assert.equal(r.error_code, 'AUTH_ERROR');
    assert.equal(r.error_reason, 'auth_error');
  });

  // Google OAuth
  it("OAuth 'invalid_grant' → auth_error (case-insensitive)", () => {
    const r = sanitizeProviderError({ raw_provider_code: 'invalid_grant' });
    assert.equal(r.error_reason, 'auth_error');
  });

  it("OAuth 'invalid_client' → auth_error", () => {
    const r = sanitizeProviderError({ raw_provider_code: 'invalid_client' });
    assert.equal(r.error_reason, 'auth_error');
  });

  it("OAuth 'invalid_scope' → invalid_argument", () => {
    const r = sanitizeProviderError({ raw_provider_code: 'invalid_scope' });
    assert.equal(r.error_reason, 'invalid_argument');
  });

  // Slack
  it("Slack 'invalid_signature' → auth_error", () => {
    const r = sanitizeProviderError({ raw_provider_code: 'invalid_signature' });
    assert.equal(r.error_reason, 'auth_error');
  });

  it("Slack 'timestamp_out_of_window' → auth_error", () => {
    const r = sanitizeProviderError({ raw_provider_code: 'timestamp_out_of_window' });
    assert.equal(r.error_reason, 'auth_error');
  });

  // v0.5.14 ack-throw replacement
  it("v0.5.14 'send_throw' → temporary_provider_error", () => {
    const r = sanitizeProviderError({ raw_provider_code: 'send_throw' });
    assert.equal(r.error_code, 'TEMPORARY_PROVIDER_ERROR');
    assert.equal(r.error_reason, 'temporary_provider_error');
  });
});

describe('sanitizeProviderError — unknown well-shaped code → passthrough + generic reason', () => {
  it("'SOMETHING_NEW' (UPPER_SNAKE) → passes through code, generic reason", () => {
    const r = sanitizeProviderError({ raw_provider_code: 'SOMETHING_NEW' });
    assert.equal(r.error_code, 'SOMETHING_NEW');
    assert.equal(
      r.error_reason,
      'provider_error',
      'unknown codes MUST map to generic reason — never raw text'
    );
  });

  it("'something_new' lowercase → normalized to UPPER_SNAKE and passes through", () => {
    const r = sanitizeProviderError({ raw_provider_code: 'something_new' });
    assert.equal(r.error_code, 'SOMETHING_NEW');
    assert.equal(r.error_reason, 'provider_error');
  });

  it('long code is truncated to 64 chars and stays generic', () => {
    const long = 'A'.repeat(80);
    const r = sanitizeProviderError({ raw_provider_code: long });
    assert.equal(r.error_code.length, 64);
    assert.equal(r.error_reason, 'provider_error');
  });
});

describe('sanitizeProviderError — non-token-shaped code is IGNORED (deny-by-default)', () => {
  // Per the founder rule: raw provider text NEVER appears in error_code.
  it("'Your message was declined' (free-text) → ignored; falls through to UNKNOWN", () => {
    const r = sanitizeProviderError({ raw_provider_code: 'Your message was declined' });
    // Falls through to the unknown_error path because no other hints provided.
    assert.equal(r.error_code, 'UNKNOWN');
    assert.equal(r.error_reason, 'unknown_error');
  });

  it("starts-with-digit code → ignored", () => {
    const r = sanitizeProviderError({ raw_provider_code: '4xx_error' });
    assert.equal(r.error_code, 'UNKNOWN');
  });

  it("special characters in code ('OAuth-Error!') → REJECTED at the input shape check (deny-by-default)", () => {
    // The exclamation mark fails the strict PROVIDER_CODE_INPUT_PATTERN
    // (which allows only letters/digits/underscore/hyphen). The input is
    // ignored entirely; fall-through path returns UNKNOWN.
    const r = sanitizeProviderError({ raw_provider_code: 'OAuth-Error!' });
    assert.equal(r.error_code, 'UNKNOWN');
    assert.equal(r.error_reason, 'unknown_error');
  });

  it("hyphen-shaped code ('OAuth-Error') → accepted, normalized to underscores", () => {
    // No exclamation: the hyphen-only variant DOES pass the strict pattern
    // and is normalized to 'OAUTH_ERROR'. Unknown to the map → generic reason.
    const r = sanitizeProviderError({ raw_provider_code: 'OAuth-Error' });
    assert.equal(r.error_code, 'OAUTH_ERROR');
    assert.equal(r.error_reason, 'provider_error');
  });

  it("empty string code → ignored; falls through", () => {
    const r = sanitizeProviderError({ raw_provider_code: '' });
    assert.equal(r.error_code, 'UNKNOWN');
  });

  it("whitespace-only code → ignored; falls through", () => {
    const r = sanitizeProviderError({ raw_provider_code: '   ' });
    assert.equal(r.error_code, 'UNKNOWN');
  });

  it('null code → ignored; falls through', () => {
    const r = sanitizeProviderError({ raw_provider_code: null });
    assert.equal(r.error_code, 'UNKNOWN');
  });
});

describe('sanitizeProviderError — network error code classification', () => {
  for (const code of [
    'ECONNRESET',
    'ETIMEDOUT',
    'ECONNREFUSED',
    'ENOTFOUND',
    'EAI_AGAIN',
    'EPIPE',
    'EHOSTUNREACH',
    'ENETUNREACH',
    'ENETDOWN',
    'EPROTO'
  ]) {
    it(`${code} → network_error`, () => {
      const r = sanitizeProviderError({ network_error_code: code });
      assert.equal(r.error_code, code);
      assert.equal(r.error_reason, 'network_error');
    });
  }

  it("unknown network code 'EUNKNOWN' → ignored; falls through to unknown_error", () => {
    const r = sanitizeProviderError({ network_error_code: 'EUNKNOWN' });
    assert.equal(r.error_code, 'UNKNOWN');
    assert.equal(r.error_reason, 'unknown_error');
  });

  it('lowercase network code → uppercased and classified', () => {
    const r = sanitizeProviderError({ network_error_code: 'econnreset' });
    assert.equal(r.error_code, 'ECONNRESET');
    assert.equal(r.error_reason, 'network_error');
  });
});

describe('sanitizeProviderError — HTTP status classification (locked table)', () => {
  it('400 → invalid_argument', () => {
    assert.equal(sanitizeProviderError({ http_status: 400 }).error_reason, 'invalid_argument');
  });
  it('401 → auth_error', () => {
    assert.equal(sanitizeProviderError({ http_status: 401 }).error_reason, 'auth_error');
  });
  it('403 → auth_error', () => {
    assert.equal(sanitizeProviderError({ http_status: 403 }).error_reason, 'auth_error');
  });
  it('404 → not_found', () => {
    assert.equal(sanitizeProviderError({ http_status: 404 }).error_reason, 'not_found');
  });
  it('408 → rate_limited (treats timeout as quota-style retry signal)', () => {
    assert.equal(sanitizeProviderError({ http_status: 408 }).error_reason, 'rate_limited');
  });
  it('422 → invalid_argument', () => {
    assert.equal(sanitizeProviderError({ http_status: 422 }).error_reason, 'invalid_argument');
  });
  it('429 → rate_limited', () => {
    assert.equal(sanitizeProviderError({ http_status: 429 }).error_reason, 'rate_limited');
  });
  it('499 (other 4xx) → invalid_argument', () => {
    assert.equal(sanitizeProviderError({ http_status: 499 }).error_reason, 'invalid_argument');
  });
  it('500 → temporary_provider_error', () => {
    assert.equal(sanitizeProviderError({ http_status: 500 }).error_reason, 'temporary_provider_error');
  });
  it('502 → temporary_provider_error', () => {
    assert.equal(sanitizeProviderError({ http_status: 502 }).error_reason, 'temporary_provider_error');
  });
  it('599 → temporary_provider_error', () => {
    assert.equal(sanitizeProviderError({ http_status: 599 }).error_reason, 'temporary_provider_error');
  });
  it('200 (operator misuse) → falls through to unknown', () => {
    assert.equal(sanitizeProviderError({ http_status: 200 }).error_reason, 'unknown_error');
  });
});

describe('sanitizeProviderError — raw_provider_message is NEVER inspected', () => {
  // Founder deny-by-default rule: the function MUST NOT pattern-match on
  // raw_provider_message content. Only presence/absence elevates the
  // fallback reason.

  it('non-empty raw message + no other hints → provider_error (presence-only signal)', () => {
    const r = sanitizeProviderError({
      raw_provider_message: 'Your message has been declined because the user has opted out'
    });
    assert.equal(r.error_code, 'PROVIDER_ERROR');
    assert.equal(r.error_reason, 'provider_error');
    // The raw text "user has opted out" MUST NOT cause us to return
    // recipient_opted_out — that would imply content matching.
    assert.notEqual(r.error_reason, 'recipient_opted_out');
  });

  it('raw message containing email/phone/token is NEVER reflected in output', () => {
    const sensitive =
      "Your message to user@gmail.com from +15551234567 failed. Bearer ya29.AbC.tokenStuff. Stack at line 42.";
    const r = sanitizeProviderError({ raw_provider_message: sensitive });
    // Output is the generic PROVIDER_ERROR row regardless of input content.
    assert.equal(r.error_code, 'PROVIDER_ERROR');
    assert.equal(r.error_reason, 'provider_error');
    // Spot-check: output contains none of the input PII fragments.
    const out = `${r.error_code} ${r.error_reason}`;
    assert.equal(out.includes('@gmail.com'), false);
    assert.equal(out.includes('+15551234567'), false);
    assert.equal(out.includes('ya29'), false);
    assert.equal(out.includes('line 42'), false);
  });

  it('empty raw message + no other hints → unknown_error', () => {
    assert.equal(
      sanitizeProviderError({ raw_provider_message: '' }).error_reason,
      'unknown_error'
    );
  });

  it('whitespace-only raw message → unknown_error', () => {
    assert.equal(
      sanitizeProviderError({ raw_provider_message: '   \n  ' }).error_reason,
      'unknown_error'
    );
  });

  it('null raw message → unknown_error', () => {
    assert.equal(
      sanitizeProviderError({ raw_provider_message: null }).error_reason,
      'unknown_error'
    );
  });
});

describe('sanitizeProviderError — decision-order precedence', () => {
  // Documented order: provider_code → network_code → http_status → raw_message → unknown.

  it('known provider_code beats network_code + http_status + raw_message', () => {
    const r = sanitizeProviderError({
      raw_provider_code: 'OPTED_OUT',
      network_error_code: 'ECONNRESET',
      http_status: 500,
      raw_provider_message: 'something else'
    });
    assert.equal(r.error_code, 'OPTED_OUT');
    assert.equal(r.error_reason, 'recipient_opted_out');
  });

  it('unknown well-shaped provider_code beats network_code + http_status', () => {
    const r = sanitizeProviderError({
      raw_provider_code: 'NEW_PROVIDER_TOKEN',
      network_error_code: 'ECONNRESET',
      http_status: 500
    });
    assert.equal(r.error_code, 'NEW_PROVIDER_TOKEN');
    assert.equal(r.error_reason, 'provider_error');
  });

  it('non-token provider_code (rejected at shape check) → falls to network_code', () => {
    // 'Free text not a token' contains spaces — fails the strict input
    // pattern. The code field is ignored entirely; we fall to network_code.
    const r = sanitizeProviderError({
      raw_provider_code: 'Free text not a token',
      network_error_code: 'ECONNRESET'
    });
    assert.equal(r.error_code, 'ECONNRESET');
    assert.equal(r.error_reason, 'network_error');
  });

  it('non-token code + unknown network code → falls to http_status', () => {
    const r = sanitizeProviderError({
      raw_provider_code: 'Free text not a token',
      network_error_code: 'EWEIRD',
      http_status: 503
    });
    assert.equal(r.error_reason, 'temporary_provider_error');
  });

  it('all hints empty/invalid → unknown_error', () => {
    const r = sanitizeProviderError({});
    assert.equal(r.error_code, 'UNKNOWN');
    assert.equal(r.error_reason, 'unknown_error');
  });
});

describe('sanitizeProviderError — privacy canary on output (deny-by-default)', () => {
  // The output of the function MUST NEVER contain any of these substrings
  // regardless of input. The function is pure + the locked tables contain
  // none of these tokens; this is a paranoia test that catches future
  // additions to the tables that violate the rule.

  const FORBIDDEN_OUTPUT_SUBSTRINGS = [
    '@gmail.com',
    '@icloud.com',
    '@hotmail.com',
    '@yahoo.com',
    '@example.com',
    '+1555',
    '+1',
    'Bearer ',
    'ya29.',
    'sb-',
    'noreply@',
    'unsubscribe',
    'Subject:',
    'From:',
    'To:',
    'stack',
    'trace',
    'cookie',
    'token',
    'password'
  ] as const;

  function check(out: { error_code: string; error_reason: string }): void {
    const blob = `${out.error_code} ${out.error_reason}`.toLowerCase();
    for (const needle of FORBIDDEN_OUTPUT_SUBSTRINGS) {
      assert.equal(
        blob.includes(needle.toLowerCase()),
        false,
        `output "${blob}" must not contain forbidden substring "${needle}"`
      );
    }
  }

  it('all KNOWN_PROVIDER_CODE_MAP entries pass the canary', () => {
    for (const v of Object.values(_KNOWN_PROVIDER_CODE_MAP_FOR_TESTS)) {
      check(v);
    }
  });

  it('all KNOWN_NETWORK_CODES outputs pass the canary', () => {
    for (const c of _KNOWN_NETWORK_CODES_FOR_TESTS) {
      check(sanitizeProviderError({ network_error_code: c }));
    }
  });

  it('HTTP status sweep [200, 599] passes the canary', () => {
    for (let s = 200; s <= 599; s++) {
      check(sanitizeProviderError({ http_status: s }));
    }
  });

  it('raw message containing all forbidden substrings still produces a canary-clean output', () => {
    // The attacker provides PII-rich text; the function output is the
    // generic PROVIDER_ERROR row, which by construction contains no PII.
    const dirty =
      "Bearer ya29.token_abc cookies and password=1234 to user@gmail.com +15551234567 Subject: stack trace at line 99";
    check(sanitizeProviderError({ raw_provider_message: dirty }));
  });
});

describe('sanitizeProviderError — bounded fields', () => {
  it('error_code is always ≤ 64 chars', () => {
    const long = 'X'.repeat(200);
    const r = sanitizeProviderError({ raw_provider_code: long });
    assert.ok(r.error_code.length <= 64);
  });

  it('error_reason is always one of the locked SanitizedReasons (closed type)', () => {
    const samples: Array<{ hint: Parameters<typeof sanitizeProviderError>[0]; }> = [
      { hint: {} },
      { hint: { raw_provider_code: 'OPTED_OUT' } },
      { hint: { raw_provider_code: 'unknown_token_thing' } },
      { hint: { network_error_code: 'ECONNRESET' } },
      { hint: { http_status: 500 } },
      { hint: { raw_provider_message: 'anything' } }
    ];
    const validReasons = new Set<SanitizedReason>(SANITIZED_REASONS);
    for (const s of samples) {
      const r = sanitizeProviderError(s.hint);
      assert.equal(
        validReasons.has(r.error_reason),
        true,
        `error_reason "${r.error_reason}" must be in the locked set`
      );
    }
  });

  it('returned object is frozen', () => {
    const r = sanitizeProviderError({ raw_provider_code: 'OPTED_OUT' });
    assert.throws(() => {
      (r as unknown as { error_code: string }).error_code = 'tampered';
    });
  });
});

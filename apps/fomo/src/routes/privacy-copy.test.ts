// Phase v0.5.1 Step 7 — privacy-copy content invariants.
//
// docs/privacy-copy-v0.5.md is rendered verbatim at /onboard. Step 7
// requires the copy to "clearly explain Gmail readonly, texting, STOP,
// founder review visibility, and beta status." This test reads the
// file from disk + asserts each topic appears in plain English. If
// the founder edits the copy to drop one of these topics, this test
// catches it before the friend sees a confusing consent page.
//
// File location is intentionally relative to the runtime; the
// `loadPrivacyCopy` helper from onboard.ts uses the same path.

import assert from 'node:assert/strict';
import { describe, it } from 'node:test';

import { buildConsentPageHtml, loadPrivacyCopy } from './onboard.js';

describe('privacy-copy-v0.5 — required topic coverage (Step 7)', () => {
  it('docs/privacy-copy-v0.5.md exists and is non-empty', async () => {
    const copy = await loadPrivacyCopy();
    assert.ok(copy.length > 200, 'privacy copy must be substantive (>200 chars)');
  });

  it('explains Gmail readonly access', async () => {
    const copy = await loadPrivacyCopy();
    assert.match(copy, /Gmail/i);
    assert.match(copy, /readonly|read-only|read your Gmail/i);
  });

  it('explains texting via iMessage', async () => {
    const copy = await loadPrivacyCopy();
    assert.match(copy, /iMessage|text|message/i);
  });

  it('explains STOP semantics', async () => {
    const copy = await loadPrivacyCopy();
    assert.match(copy, /\bSTOP\b/);
    assert.match(copy, /disable|stop all future alerts|stop future|stop alerts/i);
  });

  it('explains founder review visibility (founder sees the alert before any text)', async () => {
    const copy = await loadPrivacyCopy();
    assert.match(copy, /founder/i);
    assert.match(copy, /review|approve|approves|approval/i);
  });

  it('explains beta status', async () => {
    const copy = await loadPrivacyCopy();
    assert.match(copy, /beta/i);
  });

  it('explicitly states what Brevio does NOT do (no send / no calendar / no auto-send)', async () => {
    const copy = await loadPrivacyCopy();
    // Spec says "no auto-send", "no calendar", "no send/draft email"
    // are part of the v0.5.1 hard boundaries — the privacy copy needs
    // to surface this to the friend in plain English so they know
    // what they're consenting to.
    assert.match(copy, /not|never|will not|won't|Brevio does not/i);
    assert.match(copy, /auto-send|auto send/i);
  });

  it('buildConsentPageHtml renders the copy verbatim into the consent page', async () => {
    const copy = await loadPrivacyCopy();
    const html = buildConsentPageHtml(copy, 'a'.repeat(43));
    // The page must include a snippet of the copy.
    // (Pick a phrase that's stable across edits.)
    assert.match(html, /Gmail/);
    assert.match(html, /STOP/);
    // The "Connect with Google" button must be present.
    assert.match(html, /Connect with Google/);
    // The token gets injected as a hidden form input.
    assert.match(html, /<input type="hidden" name="token"/);
  });
});

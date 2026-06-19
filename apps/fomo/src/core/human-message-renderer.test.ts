// Phase v0.5.7 — renderHumanMessage test suite.
//
// Load-bearing for:
//   * Q1.A two-sentence template
//   * Q3.B subject-strip rules (bracket / Re: / Fwd: / multiple / none / empty)
//   * Q5.A degradation matrix (sender→generic, subject_empty, reason fallback, legacy_3p)
//   * Q6.A audit-field outputs (sender_resolution_path, subject_strip_applied,
//     reason_voice, template_shape)
//   * 220–280 / 320 / 340 length policy carry-forward from v0.5.6
//   * No arbitrary ellipsis (sentence-boundary truncation)
//   * 3E.1 PRESERVED — the renderer module imports NO LLM client.

import { describe, it } from 'node:test';
import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';
import { fileURLToPath } from 'node:url';

import {
  HUMAN_MESSAGE_ABSOLUTE_MAX_CHARS,
  HUMAN_MESSAGE_HARD_MAX_CHARS,
  HUMAN_MESSAGE_TEMPLATE_VERSION,
  RANKER_V2_PROMPT_VERSION,
  REASON_FALLBACK_STRING,
  REASON_HARD_CAP_FOR_RENDER,
  renderHumanMessage,
  stripSubject
} from './human-message-renderer.js';
import { type HumanMessageEgressView } from './egress-policy.js';

function makeView(overrides: Partial<HumanMessageEgressView> = {}): HumanMessageEgressView {
  return {
    purpose: 'human_message_renderer',
    sender_name: 'Galiette Mita',
    sender_email: 'galiettemita@icloud.com',
    subject: 'Q3 board deck final draft',
    received_at: '2026-06-06T01:24:57Z',
    message_id: 'msg-test-1',
    ...overrides
  };
}

const VALID_RANK = {
  label: 'important' as const,
  score: 0.93,
  reason: 'Mark needs your sign-off on the Q3 board deck by tomorrow.'
};

describe('renderHumanMessage — Q1.A two-sentence shape (happy path)', () => {
  it('produces the canonical "<Sender> emailed you about <subject>. <Why>." shape', () => {
    const out = renderHumanMessage({
      surface: 'email_alert',
      view: makeView({ sender_name: 'Mark Chen', sender_email: 'mark.chen@acme.com', subject: 'Q3 board deck final draft' }),
      rank: VALID_RANK,
      prompt_version: RANKER_V2_PROMPT_VERSION
    });
    assert.equal(
      out.text,
      'Mark emailed you about "Q3 board deck final draft". Mark needs your sign-off on the Q3 board deck by tomorrow.'
    );
    assert.equal(out.template_shape, 'two_sentence');
    assert.equal(out.template_version, HUMAN_MESSAGE_TEMPLATE_VERSION);
  });

  it('uses HUMAN_MESSAGE_TEMPLATE_VERSION = "human-message-v0.3.0"', () => {
    assert.equal(HUMAN_MESSAGE_TEMPLATE_VERSION, 'human-message-v0.3.0');
  });

  it('appends "." to a reason that does not end with terminator', () => {
    const out = renderHumanMessage({
      surface: 'email_alert',
      view: makeView({ sender_name: 'Sarah', sender_email: 'sarah@example.com', subject: 'Hi' }),
      rank: { label: 'important', score: 0.9, reason: 'Sarah needs you to send the form tonight' },
      prompt_version: RANKER_V2_PROMPT_VERSION
    });
    assert.ok(out.text.endsWith('tonight.'), `expected text to end with "tonight." but got: ${out.text}`);
  });

  it('preserves existing terminator (? !)', () => {
    const out = renderHumanMessage({
      surface: 'email_alert',
      view: makeView({ sender_name: 'Mark', sender_email: 'mark@example.com', subject: 'Update' }),
      rank: { label: 'important', score: 0.9, reason: 'Can you confirm tonight?' },
      prompt_version: RANKER_V2_PROMPT_VERSION
    });
    assert.ok(out.text.endsWith('tonight?'));
  });
});

describe('renderHumanMessage — Q3.B subject strip rules', () => {
  it('subject "Q3 update" → "none"', () => {
    const out = renderHumanMessage({
      surface: 'email_alert',
      view: makeView({ subject: 'Q3 update' }),
      rank: VALID_RANK,
      prompt_version: RANKER_V2_PROMPT_VERSION
    });
    assert.equal(out.subject_strip_applied, 'none');
    assert.ok(out.text.includes('"Q3 update"'));
  });

  it('subject "[v0.5.7-smoke] Q3 update" → "bracket_prefix"', () => {
    const out = renderHumanMessage({
      surface: 'email_alert',
      view: makeView({ subject: '[v0.5.7-smoke] Q3 update' }),
      rank: VALID_RANK,
      prompt_version: RANKER_V2_PROMPT_VERSION
    });
    assert.equal(out.subject_strip_applied, 'bracket_prefix');
    assert.ok(out.text.includes('"Q3 update"'));
    assert.ok(!out.text.includes('[v0.5.7-smoke]'));
  });

  it('subject "Re: Tuesday meeting" → "re_fwd"', () => {
    const out = renderHumanMessage({
      surface: 'email_alert',
      view: makeView({ subject: 'Re: Tuesday meeting' }),
      rank: VALID_RANK,
      prompt_version: RANKER_V2_PROMPT_VERSION
    });
    assert.equal(out.subject_strip_applied, 're_fwd');
    assert.ok(out.text.includes('"Tuesday meeting"'));
  });

  it('subject "FW: Re: Q3 update" → "re_fwd" (both Fwd and Re stripped)', () => {
    const out = renderHumanMessage({
      surface: 'email_alert',
      view: makeView({ subject: 'FW: Re: Q3 update' }),
      rank: VALID_RANK,
      prompt_version: RANKER_V2_PROMPT_VERSION
    });
    assert.equal(out.subject_strip_applied, 're_fwd');
    assert.ok(out.text.includes('"Q3 update"'));
  });

  it('subject "[urgent] Re: Q3" → "multiple"', () => {
    const out = renderHumanMessage({
      surface: 'email_alert',
      view: makeView({ subject: '[urgent] Re: Q3' }),
      rank: VALID_RANK,
      prompt_version: RANKER_V2_PROMPT_VERSION
    });
    assert.equal(out.subject_strip_applied, 'multiple');
    assert.ok(out.text.includes('"Q3"'));
    assert.ok(!out.text.includes('[urgent]'));
    assert.ok(!out.text.toLowerCase().includes(' re:'));
  });

  it('subject "" → "subject_empty" + single-sentence shape', () => {
    const out = renderHumanMessage({
      surface: 'email_alert',
      view: makeView({ sender_name: 'Sarah', sender_email: 'sarah@example.com', subject: '' }),
      rank: VALID_RANK,
      prompt_version: RANKER_V2_PROMPT_VERSION
    });
    assert.equal(out.subject_strip_applied, 'subject_empty');
    assert.equal(out.template_shape, 'single_sentence_no_subject');
    assert.equal(out.degradation_applied, true);
    assert.ok(out.text.startsWith('Sarah emailed you. '));
    // Should NOT contain "about" since subject was empty
    assert.ok(!out.text.includes('about'));
  });

  it('subject "[bracketed only]" → empties out → "subject_empty"', () => {
    const out = renderHumanMessage({
      surface: 'email_alert',
      view: makeView({ subject: '[v0.5.7-smoke]' }),
      rank: VALID_RANK,
      prompt_version: RANKER_V2_PROMPT_VERSION
    });
    assert.equal(out.subject_strip_applied, 'subject_empty');
    assert.equal(out.template_shape, 'single_sentence_no_subject');
  });

  it('stripSubject — exposed helper covers all 5 enum values', () => {
    assert.deepEqual(stripSubject('Q3 update'), { stripped: 'Q3 update', applied: 'none' });
    assert.deepEqual(stripSubject('[urgent] Q3'), { stripped: 'Q3', applied: 'bracket_prefix' });
    assert.deepEqual(stripSubject('Re: Q3'), { stripped: 'Q3', applied: 're_fwd' });
    assert.deepEqual(stripSubject('[urgent] Re: Q3'), { stripped: 'Q3', applied: 'multiple' });
    assert.deepEqual(stripSubject(''), { stripped: '', applied: 'subject_empty' });
    assert.deepEqual(stripSubject(undefined), { stripped: '', applied: 'subject_empty' });
    assert.deepEqual(stripSubject('   '), { stripped: '', applied: 'subject_empty' });
  });
});

describe('renderHumanMessage — Q4.A reason voice routing', () => {
  it('prompt_version = ranker-v0.2.0 → reason_voice = "2p_action"', () => {
    const out = renderHumanMessage({
      surface: 'email_alert',
      view: makeView(),
      rank: VALID_RANK,
      prompt_version: 'ranker-v0.2.0'
    });
    assert.equal(out.reason_voice, '2p_action');
    assert.equal(out.degradation_applied, false);
  });

  it('prompt_version = ranker-v0.1.0 → reason_voice = "legacy_3p" + degradation_applied=true', () => {
    const out = renderHumanMessage({
      surface: 'email_alert',
      view: makeView(),
      rank: { label: 'important', score: 0.9, reason: 'Time-sensitive sign-off request from colleague/manager.' },
      prompt_version: 'ranker-v0.1.0'
    });
    assert.equal(out.reason_voice, 'legacy_3p');
    assert.equal(out.degradation_applied, true);
    // Body still uses the reason verbatim even though voice is legacy.
    assert.ok(out.text.includes('Time-sensitive sign-off request'));
  });

  it('unknown prompt_version → "legacy_3p" (treated as pre-v0.2.0)', () => {
    const out = renderHumanMessage({
      surface: 'email_alert',
      view: makeView(),
      rank: VALID_RANK,
      prompt_version: 'ranker-v0.0.1-experimental'
    });
    assert.equal(out.reason_voice, 'legacy_3p');
  });
});

describe('renderHumanMessage — Q5.A degradation matrix', () => {
  it('sender_resolution_path="first_name" + valid reason → degradation_applied=false', () => {
    const out = renderHumanMessage({
      surface: 'email_alert',
      view: makeView({ sender_name: 'Mark', sender_email: 'mark@example.com' }),
      rank: VALID_RANK,
      prompt_version: RANKER_V2_PROMPT_VERSION
    });
    assert.equal(out.sender_resolution_path, 'first_name');
    assert.equal(out.degradation_applied, false);
  });

  it('sender → "generic" (Someone) → degradation_applied=true', () => {
    const out = renderHumanMessage({
      surface: 'email_alert',
      view: makeView({
        sender_name: undefined,
        sender_email: 'galiettemita@uncurated-personal.io'
      }),
      rank: VALID_RANK,
      prompt_version: RANKER_V2_PROMPT_VERSION
    });
    assert.equal(out.sender_resolution_path, 'generic');
    assert.equal(out.degradation_applied, true);
    assert.ok(out.text.startsWith('Someone emailed you about '));
  });

  it('empty reason → reason_voice="fallback" + template_shape="fallback_string"', () => {
    const out = renderHumanMessage({
      surface: 'email_alert',
      view: makeView(),
      rank: { label: 'important', score: 0.9, reason: '' },
      prompt_version: RANKER_V2_PROMPT_VERSION
    });
    assert.equal(out.reason_voice, 'fallback');
    assert.equal(out.reason_violation_kind, 'empty');
    assert.equal(out.template_shape, 'fallback_string');
    assert.equal(out.degradation_applied, true);
    assert.ok(out.text.includes(REASON_FALLBACK_STRING));
    assert.equal(out.original_reason_length, 0);
  });

  it('whitespace-only reason → reason_voice="fallback" + violation="empty"', () => {
    const out = renderHumanMessage({
      surface: 'email_alert',
      view: makeView(),
      rank: { label: 'important', score: 0.9, reason: '   \n\t ' },
      prompt_version: RANKER_V2_PROMPT_VERSION
    });
    assert.equal(out.reason_voice, 'fallback');
    assert.equal(out.reason_violation_kind, 'empty');
  });

  it('over-180 reason → reason_voice="fallback" + violation="too_long" + original_reason_length captured', () => {
    const longReason = 'x'.repeat(250);
    const out = renderHumanMessage({
      surface: 'email_alert',
      view: makeView(),
      rank: { label: 'important', score: 0.9, reason: longReason },
      prompt_version: RANKER_V2_PROMPT_VERSION
    });
    assert.equal(out.reason_voice, 'fallback');
    assert.equal(out.reason_violation_kind, 'too_long');
    assert.equal(out.original_reason_length, 250);
    assert.ok(out.text.includes(REASON_FALLBACK_STRING));
    assert.ok(!out.text.includes('xxx'), 'rendered body must not include the long-reason payload');
  });

  it('valid reason exactly at REASON_HARD_CAP_FOR_RENDER (180) passes', () => {
    const reason180 = 'X' + 'a'.repeat(REASON_HARD_CAP_FOR_RENDER - 2) + '.';
    assert.equal(reason180.length, REASON_HARD_CAP_FOR_RENDER);
    const out = renderHumanMessage({
      surface: 'email_alert',
      view: makeView(),
      rank: { label: 'important', score: 0.9, reason: reason180 },
      prompt_version: RANKER_V2_PROMPT_VERSION
    });
    assert.equal(out.reason_voice, '2p_action');
    assert.equal(out.reason_violation_kind, null);
  });

  it('REASON_FALLBACK_STRING is the v0.5.6 carry-forward string', () => {
    assert.equal(REASON_FALLBACK_STRING, 'Marked important by Brevio.');
  });
});

describe('renderHumanMessage — length policy (carry-forward from v0.5.6)', () => {
  it('short body passes through unchanged (well under hard cap)', () => {
    const out = renderHumanMessage({
      surface: 'email_alert',
      view: makeView({ sender_name: 'Mark', sender_email: 'mark@example.com', subject: 'Q3' }),
      rank: { label: 'important', score: 0.9, reason: 'Sign off tonight.' },
      prompt_version: RANKER_V2_PROMPT_VERSION
    });
    assert.ok(out.text.length < HUMAN_MESSAGE_HARD_MAX_CHARS);
    assert.equal(out.text, 'Mark emailed you about "Q3". Sign off tonight.');
  });

  it('body over HARD_MAX (320) truncates the reason at a sentence boundary, no ellipsis', () => {
    const reasonOver = 'Mark needs sign-off. ' + 'Many other clauses with extra context. '.repeat(5);
    const out = renderHumanMessage({
      surface: 'email_alert',
      view: makeView({
        sender_name: 'Mark',
        sender_email: 'mark@example.com',
        subject: 'Q3 board deck final draft with many words'
      }),
      rank: { label: 'important', score: 0.9, reason: reasonOver },
      prompt_version: RANKER_V2_PROMPT_VERSION
    });
    assert.ok(out.text.length <= HUMAN_MESSAGE_HARD_MAX_CHARS, `text length ${out.text.length} > ${HUMAN_MESSAGE_HARD_MAX_CHARS}`);
    assert.ok(!out.text.includes('…'), `no arbitrary ellipsis allowed: ${out.text}`);
  });

  it('NEVER appends ellipsis under any input', () => {
    const cases = [
      'A short reason',
      'A long reason that should not get cut off mid-sentence and definitely not append ellipsis.',
      'A reason that ends with a question?'
    ];
    for (const reason of cases) {
      const out = renderHumanMessage({
        surface: 'email_alert',
        view: makeView(),
        rank: { label: 'important', score: 0.9, reason },
        prompt_version: RANKER_V2_PROMPT_VERSION
      });
      assert.ok(!out.text.includes('…'), `ellipsis detected in: ${out.text}`);
    }
  });

  it('absolute-cap defense: even pathological inputs never exceed ABSOLUTE_MAX', () => {
    const out = renderHumanMessage({
      surface: 'email_alert',
      view: makeView({
        sender_name: 'Mark',
        sender_email: 'mark@example.com',
        subject: 'A'.repeat(500)
      }),
      rank: VALID_RANK,
      prompt_version: RANKER_V2_PROMPT_VERSION
    });
    assert.ok(out.text.length <= HUMAN_MESSAGE_ABSOLUTE_MAX_CHARS);
  });
});

describe('renderHumanMessage — Q6.A audit field outputs (4 fields)', () => {
  it('happy path produces all 4 audit fields with expected enums', () => {
    const out = renderHumanMessage({
      surface: 'email_alert',
      view: makeView({ sender_name: 'Mark', sender_email: 'mark@example.com', subject: 'Q3' }),
      rank: VALID_RANK,
      prompt_version: RANKER_V2_PROMPT_VERSION
    });
    assert.equal(out.sender_resolution_path, 'first_name');
    assert.equal(out.subject_strip_applied, 'none');
    assert.equal(out.reason_voice, '2p_action');
    assert.equal(out.template_shape, 'two_sentence');
    assert.equal(out.degradation_applied, false);
  });

  it('worst-case path produces all 4 degraded audit fields', () => {
    const out = renderHumanMessage({
      surface: 'email_alert',
      view: makeView({
        sender_name: undefined,
        sender_email: 'galiettemita@uncurated-personal.io',
        subject: '[v0.5.7-smoke]'
      }),
      rank: { label: 'important', score: 0.9, reason: '' },
      prompt_version: 'ranker-v0.1.0'
    });
    assert.equal(out.sender_resolution_path, 'generic');
    assert.equal(out.subject_strip_applied, 'subject_empty');
    assert.equal(out.reason_voice, 'fallback');
    assert.equal(out.template_shape, 'fallback_string');
    assert.equal(out.degradation_applied, true);
    assert.equal(out.reason_violation_kind, 'empty');
    assert.equal(out.original_reason_length, 0);
  });
});

describe('renderHumanMessage — output privacy invariants', () => {
  it('NEVER includes the raw email address in output text', () => {
    const out = renderHumanMessage({
      surface: 'email_alert',
      view: makeView({
        sender_name: 'Galiette Mita',
        sender_email: 'galiettemita@icloud.com'
      }),
      rank: VALID_RANK,
      prompt_version: RANKER_V2_PROMPT_VERSION
    });
    assert.ok(!out.text.includes('galiettemita@icloud.com'));
    assert.ok(!out.text.includes('@icloud.com'));
    // Also no masked email shape in the opener (founder lock).
    assert.ok(!out.text.includes('***@'));
  });

  it('NEVER includes the literal subject prefix once stripped', () => {
    const out = renderHumanMessage({
      surface: 'email_alert',
      view: makeView({ subject: '[v0.5.7-smoke] Q3 update' }),
      rank: VALID_RANK,
      prompt_version: RANKER_V2_PROMPT_VERSION
    });
    assert.ok(!out.text.includes('[v0.5.7-smoke]'));
  });
});

describe('renderHumanMessage — 3E.1 PRESERVED (no LLM body generation)', () => {
  // Load-bearing assertion per memory feedback_3e1-no-llm-body-generation.
  // If anyone introduces an LLM/OpenAI/Anthropic import in this module,
  // this test fails — tripwire for 3E.1 reversal.
  it('the renderer module imports NO LLM / OpenAI / Anthropic client', async () => {
    const sourcePath = fileURLToPath(new URL('./human-message-renderer.ts', import.meta.url));
    const source = await readFile(sourcePath, 'utf8');
    const forbiddenImports = [
      /from ['"]openai['"]/,
      /from ['"]@anthropic-ai\//,
      /from ['"]@google-cloud\/vertexai['"]/,
      /import\s+OpenAI\b/,
      /import\s+Anthropic\b/,
      /createCompletion/,
      /chat\.completions/,
      /messages\.create/
    ];
    for (const pat of forbiddenImports) {
      assert.equal(
        pat.test(source),
        false,
        `3E.1 INVARIANT VIOLATED: renderer module matches forbidden LLM-import pattern ${pat}`
      );
    }
  });

  it('the renderer module only imports from ./sender-resolution, ./egress-policy, and ../memory/rank-results', async () => {
    const sourcePath = fileURLToPath(new URL('./human-message-renderer.ts', import.meta.url));
    const source = await readFile(sourcePath, 'utf8');
    // Pull out all `import { … } from '…'` paths and verify none is an
    // unexpected module that could be an LLM client.
    const importRegex = /import\s+[^;]+?from\s+['"]([^'"]+)['"]/g;
    const allowed = new Set([
      './sender-resolution.js',
      './egress-policy.js',
      '../memory/rank-results.js'
    ]);
    let m: RegExpExecArray | null;
    while ((m = importRegex.exec(source)) !== null) {
      const path = m[1];
      assert.ok(
        allowed.has(path ?? ''),
        `unexpected import path "${path}" in human-message-renderer.ts — 3E.1 tripwire`
      );
    }
  });
});

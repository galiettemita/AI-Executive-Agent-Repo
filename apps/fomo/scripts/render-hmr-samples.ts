// Render HMR Samples — Phase v0.5.7 taste-check fixture.
//
// LOAD-BEARING for C10 per founder lock 2026-06-06: taste check on
// RENDERED BODIES is the load-bearing evidence path for v0.5.7 PASS,
// NOT real iMessage delivery (which is opportunistic). This script
// renders N representative bodies offline so the founder can eye-test
// the new HMR shape WITHOUT depending on SendBlue tier state.
//
// Usage:
//   pnpm --filter @brevio/fomo run render-hmr-samples
//
// Output: prints each sample's input shape (sender, subject, reason)
// followed by the rendered body + the 4 new Q6.A audit fields the
// runtime would write into fomo.send.attempted detail.
//
// 3E.1 PRESERVED — this script imports only the pure deterministic
// renderer + helpers. No DB, no network, no LLM, no clock, no random.

import {
  renderHumanMessage,
  HUMAN_MESSAGE_TEMPLATE_VERSION,
  RANKER_V2_PROMPT_VERSION,
  type HumanMessageInput
} from '../src/core/human-message-renderer.js';
import { type HumanMessageEgressView } from '../src/core/egress-policy.js';

interface Sample {
  readonly label: string;
  readonly note: string;
  readonly view: HumanMessageEgressView;
  readonly rank: {
    readonly label: 'important' | 'not_important';
    readonly score: number;
    readonly reason: string;
  };
  readonly prompt_version: string;
}

function view(o: {
  sender_name?: string;
  sender_email: string;
  subject: string;
}): HumanMessageEgressView {
  return Object.freeze({
    purpose: 'human_message_renderer',
    sender_name: o.sender_name,
    sender_email: o.sender_email,
    subject: o.subject,
    received_at: '2026-06-06T01:24:57Z',
    message_id: 'msg-hmr-fixture'
  });
}

// 5 founder-locked samples + 4 degradation-path samples (to surface
// what the user would see if Q5.A fallbacks fire).
const SAMPLES: readonly Sample[] = [
  {
    label: '1/9 — Q1.A happy path: colleague time-sensitive ask',
    note: 'first_name + none + 2p_action + two_sentence',
    view: view({
      sender_name: 'Mark Chen',
      sender_email: 'mark.chen@acme.com',
      subject: 'Q3 board deck final draft'
    }),
    rank: {
      label: 'important',
      score: 0.92,
      reason: 'Mark needs your sign-off on the Q3 board deck by tomorrow.'
    },
    prompt_version: RANKER_V2_PROMPT_VERSION
  },
  {
    label: '2/9 — Q1.A happy path: family/close-friend ask (founder anchor)',
    note: 'first_name + none + 2p_action + two_sentence',
    view: view({
      sender_name: 'Sarah Mita',
      sender_email: 'sarah.mita@icloud.com',
      subject: 'Can you send this form tonight?'
    }),
    rank: {
      label: 'important',
      score: 0.95,
      reason: 'Sarah needs you to send the form tonight.'
    },
    prompt_version: RANKER_V2_PROMPT_VERSION
  },
  {
    label: '3/9 — Q1.A happy path: counselor scheduling',
    note: 'first_name + re_fwd strip + 2p_action + two_sentence',
    view: view({
      sender_name: 'Counselor Ramos',
      sender_email: 'r.ramos@school.edu',
      subject: 'Re: College apps — Tuesday meeting'
    }),
    rank: {
      label: 'important',
      score: 0.88,
      reason: "Your counselor is confirming Tuesday's college-apps meeting."
    },
    prompt_version: RANKER_V2_PROMPT_VERSION
  },
  {
    label: '4/9 — Q1.A not_important: system transactional',
    note: 'domain_label (curated SaaS) + none + 2p_action + two_sentence',
    view: view({
      sender_email: 'no-reply@stripe.com',
      subject: 'Receipt for your $42.10 payment'
    }),
    rank: {
      label: 'not_important',
      score: 0.93,
      reason: 'Stripe receipt for a $42.10 charge — no action needed.'
    },
    prompt_version: RANKER_V2_PROMPT_VERSION
  },
  {
    label: '5/9 — Q1.A not_important: system digest',
    note: 'domain_label (curated SaaS) + none + 2p_action + two_sentence',
    view: view({
      sender_email: 'jobs-noreply@linkedin.com',
      subject: '12 new jobs match your search'
    }),
    rank: {
      label: 'not_important',
      score: 0.96,
      reason: 'Weekly LinkedIn jobs digest — nothing personal or time-sensitive.'
    },
    prompt_version: RANKER_V2_PROMPT_VERSION
  },
  {
    label: '6/9 — Q3.B strip: [bracketed] prefix + subject naturalization',
    note: 'first_name + bracket_prefix + 2p_action + two_sentence',
    view: view({
      sender_name: 'Galiette Mita',
      sender_email: 'galiettemita@icloud.com',
      subject: '[v0.5.7-smoke] Q4 board deck final draft'
    }),
    rank: {
      label: 'important',
      score: 0.91,
      reason: "Galiette's drafted the Q4 board deck and wants your eyes on it before tomorrow."
    },
    prompt_version: RANKER_V2_PROMPT_VERSION
  },
  {
    label: '7/9 — Q5.A degradation: sender → "Someone" (single-token email local)',
    note: 'generic + none + 2p_action + two_sentence — degradation_applied=true',
    view: view({
      // No sender_name; local part "galiettemita" is single-token (not
      // human-readable). The Modified Q2.B chain must fall through to
      // "Someone" per founder lock — DO NOT produce "Galiettemita".
      sender_email: 'galiettemita@uncurated-personal.io',
      subject: 'Hey'
    }),
    rank: {
      label: 'important',
      score: 0.78,
      reason: 'Sender needs you to confirm tonight.'
    },
    prompt_version: RANKER_V2_PROMPT_VERSION
  },
  {
    label: '8/9 — Q5.A degradation: empty subject → single_sentence_no_subject',
    note: 'first_name + subject_empty + 2p_action + single_sentence_no_subject',
    view: view({
      sender_name: 'Mark',
      sender_email: 'mark.chen@acme.com',
      subject: ''
    }),
    rank: {
      label: 'important',
      score: 0.85,
      reason: 'Mark wants your eyes on the contract redline by end of day.'
    },
    prompt_version: RANKER_V2_PROMPT_VERSION
  },
  {
    label: '9/9 — Q5.A degradation: reason fallback (>180 chars) → fallback_string',
    note: 'first_name + none + fallback + fallback_string',
    view: view({
      sender_name: 'Mark Chen',
      sender_email: 'mark.chen@acme.com',
      subject: 'Q3 board deck final draft'
    }),
    rank: {
      label: 'important',
      score: 0.91,
      reason: 'x'.repeat(250)
    },
    prompt_version: RANKER_V2_PROMPT_VERSION
  }
];

function renderSample(s: Sample): void {
  const input: HumanMessageInput = {
    surface: 'email_alert',
    view: s.view,
    rank: s.rank,
    prompt_version: s.prompt_version
  };
  const out = renderHumanMessage(input);
  console.log('');
  console.log('================================================================================');
  console.log(s.label);
  console.log(`  expected: ${s.note}`);
  console.log('--------------------------------------------------------------------------------');
  console.log(`  INPUT:`);
  console.log(`    sender_name:  ${JSON.stringify(s.view.sender_name)}`);
  console.log(`    sender_email: ${s.view.sender_email}`);
  console.log(`    subject:      ${JSON.stringify(s.view.subject)}`);
  console.log(`    rank.label:   ${s.rank.label}`);
  console.log(`    rank.score:   ${s.rank.score}`);
  console.log(`    rank.reason:  ${s.rank.reason.length > 60 ? `<${s.rank.reason.length} chars>` : JSON.stringify(s.rank.reason)}`);
  console.log(`    prompt_ver:   ${s.prompt_version}`);
  console.log('  RENDERED BODY (the actual iMessage text):');
  console.log('  ┌' + '─'.repeat(78) + '┐');
  console.log('  │ ' + out.text);
  console.log('  └' + '─'.repeat(78) + '┘');
  console.log(`  content_chars:           ${out.text.length}`);
  console.log(`  Q6.A AUDIT FIELDS (what outbound-sender writes to fomo.send.attempted detail):`);
  console.log(`    template_version:        ${out.template_version}`);
  console.log(`    sender_resolution_path:  ${out.sender_resolution_path}`);
  console.log(`    subject_strip_applied:   ${out.subject_strip_applied}`);
  console.log(`    reason_voice:            ${out.reason_voice}`);
  console.log(`    template_shape:          ${out.template_shape}`);
  console.log(`    degradation_applied:     ${out.degradation_applied}`);
  if (out.reason_violation_kind) {
    console.log(`    reason_violation_kind:   ${out.reason_violation_kind}`);
    console.log(`    original_reason_length:  ${out.original_reason_length}`);
  }
}

console.log('================================================================================');
console.log(`Phase v0.5.7 — Human Message Renderer taste-check fixture`);
console.log(`Template version: ${HUMAN_MESSAGE_TEMPLATE_VERSION}`);
console.log(`Locked ranker prompt version: ${RANKER_V2_PROMPT_VERSION}`);
console.log(`Renders ${SAMPLES.length} sample bodies for founder eye-test (LOAD-BEARING per C10 correction).`);
console.log('================================================================================');

console.log('');
console.log('  Founder eye-test checklist for each rendered body:');
console.log('    [ ] Reads as a natural 1–2 sentence message, NOT field-newline list');
console.log('    [ ] Opens with a person-named or domain-named sender (NOT g***@…)');
console.log('    [ ] No "FOMO · IMPORTANT (0.92)" telemetry header');
console.log('    [ ] No arbitrary "…" ellipsis');
console.log('    [ ] Subject reads cleanly (no [bracketed], no Re:/Fwd: artifacts)');
console.log('    [ ] Why-clause is 2nd-person action prose (if 2p_action) or fallback string');
console.log('    [ ] Length feels right for lock-screen reading');
console.log('    [ ] Feels like a person curated it (founder example bar)');
console.log('');
console.log('  Founder example bar:');
console.log('    "Galiette emailed you about the Q3 board deck. It looks');
console.log('     time-sensitive — she needs sign-off by tomorrow."');

for (const sample of SAMPLES) {
  renderSample(sample);
}

console.log('');
console.log('================================================================================');
console.log(`Done. ${SAMPLES.length} sample bodies rendered.`);
console.log('');
console.log('Founder action: paste each rendered body into SMOKE_REPORT_v0.5.7.md §10 with');
console.log('eye-test notes. If any sample reads as field-shaped metadata rather than a');
console.log('natural sentence, that is a Q1.A regression and v0.5.7 should NOT pass C6/C10.');
console.log('================================================================================');

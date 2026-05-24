// Synthetic ranker fixtures — Phase 3C.1.
//
// 20 hand-written email examples spanning important / not_important.
// All synthetic, all anonymized, all safe to commit per FOMO_PLAN
// §14.3 ("no raw real email bodies in repo"). Sender domains are
// generic (example.com, school.edu, hospital.org) — never real.
//
// Coverage goals:
//   * Time-sensitive personal asks (counselor, doctor, school) → important
//   * Family / friend asks → important
//   * Employer / interview / job → important
//   * Bills with deadlines → important
//   * Promotional / marketing → not_important
//   * Newsletters / digests → not_important
//   * Social network notifications → not_important
//   * Automated transactional confirmations w/o deadline → not_important
//
// The eval harness (ranker-eval.ts) builds the actual prompt at
// runtime via buildRankerPrompt, so a prompt-version bump does NOT
// require touching this file. Add new fixtures freely.

import { type RankLabel } from '../../ranker/validator.js';

export interface RankerFixture {
  readonly id: string;
  // One-line description for human readers — does not feed the model.
  readonly description: string;
  // The semantic fields. The eval harness wraps these into a
  // RawEmailContext, runs them through applyEgressForRanker, then
  // calls buildRankerPrompt.
  readonly sender_email: string;
  readonly sender_name: string | undefined;
  readonly subject: string;
  readonly body_plain: string;
  readonly has_attachments?: boolean;
  readonly attachment_count?: number;
  readonly expected_label: RankLabel;
}

export const RANKER_FIXTURES: readonly RankerFixture[] = Object.freeze([
  /* ----- IMPORTANT — time-sensitive asks from real people ----- */
  {
    id: 'fx-001-counselor-form-tonight',
    description: 'Counselor requesting form submission by tonight',
    sender_email: 'jane.smith@school.edu',
    sender_name: 'Jane Smith (Counselor)',
    subject: 'Interview form due tonight',
    body_plain:
      'Hi Albert, please submit the interview form by 9pm tonight so we can finalize tomorrow. Thanks, Jane.',
    expected_label: 'important'
  },
  {
    id: 'fx-002-doctor-test-results',
    description: 'Doctor with test results requiring action',
    sender_email: 'nurse@familyhealth.org',
    sender_name: 'Dr. Reed Office',
    subject: 'Your test results are ready — please call to discuss',
    body_plain:
      'Hi, your recent lab results are back. Please call our office at your earliest convenience to discuss next steps.',
    expected_label: 'important'
  },
  {
    id: 'fx-003-mom-needs-call-back',
    description: 'Family member asking for a call back',
    sender_email: 'mom@example.com',
    sender_name: 'Mom',
    subject: 'Call me when you can',
    body_plain: 'Sweetie, give me a call tonight when you get a chance — nothing bad, just want to talk.',
    expected_label: 'important'
  },
  {
    id: 'fx-004-interview-confirmation',
    description: 'Recruiter confirming interview slot',
    sender_email: 'recruiter@startup.example.com',
    sender_name: 'Maya at Lattice Recruiting',
    subject: 'Confirming your interview Thursday at 2pm PT',
    body_plain:
      'Hi Albert, please confirm the Thursday 2pm PT slot for your interview with the engineering team. Zoom link to follow.',
    expected_label: 'important'
  },
  {
    id: 'fx-005-rent-due-tomorrow',
    description: 'Landlord — rent due tomorrow',
    sender_email: 'manager@hilltopapts.example.com',
    sender_name: 'Hilltop Apartments',
    subject: 'Rent is due tomorrow, May 25',
    body_plain:
      'Friendly reminder: your May rent is due tomorrow. Please submit via the portal or stop by the office.',
    expected_label: 'important'
  },
  {
    id: 'fx-006-bank-fraud-alert',
    description: 'Bank fraud alert — requires action',
    sender_email: 'alerts@bank.example.com',
    sender_name: 'Bank Security',
    subject: 'Unusual sign-in attempt — review now',
    body_plain:
      'We detected a sign-in attempt from a new device. If this was not you, please secure your account immediately.',
    expected_label: 'important'
  },
  {
    id: 'fx-007-professor-deadline',
    description: 'Professor with assignment deadline today',
    sender_email: 'prof.lee@university.edu',
    sender_name: 'Prof. Lee',
    subject: 'CS401: final project submission window closes 11:59pm',
    body_plain:
      'Reminder that final project submissions close at 11:59pm tonight. No late submissions will be accepted per the syllabus.',
    expected_label: 'important'
  },
  {
    id: 'fx-008-close-friend-event-tonight',
    description: 'Close friend asking about plans tonight',
    sender_email: 'sarah.t@example.com',
    sender_name: 'Sarah T.',
    subject: 'are you coming tonight?',
    body_plain:
      'hey! show starts at 8, we have an extra ticket — text me back if you can make it!',
    expected_label: 'important'
  },
  {
    id: 'fx-009-flight-cancelled',
    description: 'Airline cancelling tomorrow flight',
    sender_email: 'noreply@airline.example.com',
    sender_name: 'Airline Notifications',
    subject: 'Action required: your flight tomorrow has been cancelled',
    body_plain:
      'We regret to inform you that your flight scheduled for May 25 has been cancelled. Please rebook through the app.',
    expected_label: 'important'
  },
  {
    id: 'fx-010-school-emergency',
    description: 'School emergency / lockdown notice',
    sender_email: 'principal@school.edu',
    sender_name: "Principal's Office",
    subject: 'Important: dismissal time change today',
    body_plain:
      'Due to a facility issue, today\'s dismissal will be 30 minutes earlier. Please arrange pickup at 2:30pm.',
    expected_label: 'important'
  },

  /* ----- NOT_IMPORTANT — marketing, digests, automated noise ----- */
  {
    id: 'fx-011-linkedin-weekly-digest',
    description: 'LinkedIn weekly job digest',
    sender_email: 'jobs-noreply@linkedin.example.com',
    sender_name: 'LinkedIn Job Alerts',
    subject: '5 new jobs that match your search',
    body_plain:
      'Here are 5 new job postings this week that match your saved search. View them on LinkedIn.',
    expected_label: 'not_important'
  },
  {
    id: 'fx-012-promo-50-off',
    description: 'Retailer 50% off sale',
    sender_email: 'deals@bigretailer.example.com',
    sender_name: 'Big Retailer',
    subject: '🔥 50% off everything this weekend only!',
    body_plain:
      'Use code WEEKEND50 at checkout for 50% off everything. Sale ends Sunday at midnight.',
    expected_label: 'not_important'
  },
  {
    id: 'fx-013-newsletter-tech-weekly',
    description: 'Generic tech newsletter',
    sender_email: 'hello@techweekly.example.com',
    sender_name: 'Tech Weekly',
    subject: 'Issue #243: what happened in AI this week',
    body_plain:
      'This week in AI: new model releases, a startup raised $50M, and a regulator issued draft guidance...',
    expected_label: 'not_important'
  },
  {
    id: 'fx-014-package-shipped',
    description: 'Automated shipment notification, no action needed',
    sender_email: 'shipment-tracking@shop.example.com',
    sender_name: 'Shop Notifications',
    subject: 'Your order has shipped',
    body_plain:
      'Good news! Your order #38291 has shipped and should arrive in 3-5 business days.',
    expected_label: 'not_important'
  },
  {
    id: 'fx-015-twitter-notification',
    description: 'Social network engagement notification',
    sender_email: 'notify@socialnet.example.com',
    sender_name: 'SocialNet',
    subject: '@friend123 liked your post',
    body_plain: 'Someone liked your recent post. Open the app to see more activity.',
    expected_label: 'not_important'
  },
  {
    id: 'fx-016-receipt-coffee',
    description: 'Receipt from coffee shop — informational only',
    sender_email: 'receipts@coffeeshop.example.com',
    sender_name: 'Coffee Shop',
    subject: 'Your receipt from May 23',
    body_plain: 'Thanks for stopping by! Your purchase total was $6.50. See you again soon.',
    expected_label: 'not_important'
  },
  {
    id: 'fx-017-saas-feature-announcement',
    description: 'SaaS new-feature announcement',
    sender_email: 'product@saastool.example.com',
    sender_name: 'SaasTool Updates',
    subject: 'Introducing: dark mode is here 🌙',
    body_plain:
      "We've launched dark mode by popular demand. Toggle it on in Settings → Appearance.",
    expected_label: 'not_important'
  },
  {
    id: 'fx-018-survey-request',
    description: 'Generic survey request',
    sender_email: 'feedback@somecompany.example.com',
    sender_name: 'Some Company',
    subject: 'How was your recent experience?',
    body_plain: 'Take 2 minutes to fill out our survey and help us improve. Thanks for your time!',
    expected_label: 'not_important'
  },
  {
    id: 'fx-019-podcast-new-episode',
    description: 'Podcast new-episode notification',
    sender_email: 'notify@podcast.example.com',
    sender_name: 'Podcast Updates',
    subject: 'New episode: Why MCP matters',
    body_plain:
      "We just dropped a new episode on Model Context Protocol. Listen wherever you get your podcasts.",
    expected_label: 'not_important'
  },
  {
    id: 'fx-020-calendar-invite-bulk',
    description: 'Generic bulk-calendar invite (low signal)',
    sender_email: 'noreply@eventbright.example.com',
    sender_name: 'Eventbright',
    subject: 'Reminder: Free Webinar — 10 Productivity Tips',
    body_plain:
      "Don't forget — the free productivity webinar starts in 3 days. RSVP if you haven't already.",
    expected_label: 'not_important'
  }
] satisfies readonly RankerFixture[]);

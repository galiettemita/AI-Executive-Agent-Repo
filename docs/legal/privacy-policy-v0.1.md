# Brevio — Privacy Policy

> **Source of truth.** This file is the canonical privacy policy text. It is published as a public page on the Brevio landing site (currently <https://orbit-landing-ten.vercel.app/privacy>) and listed as the **Privacy Policy URL** on the Google OAuth consent screen for the Brevio Google Cloud project.
>
> **Before publishing this file, reconcile the brand name.** This repo and this draft say "Brevio". The Vercel project is `orbit-landing-ten` and the support email domain is `testing-orbit.com`. Pick one public-facing name (Brevio or Orbit) and make every public surface match before submitting to Google verification.
>
> **Drafted:** 2026-06-08. **Last updated:** _set to publish date when you publish._
>
> This is not legal advice. Have whoever signs off on the rest of Brevio's legal copy run their eye over this before going live.

---

**Last updated:** _<publish date>_

This Privacy Policy describes how Brevio ("Brevio", "we", "us") collects, uses, and protects information when you use the Brevio service. By connecting a Google account to Brevio or by receiving messages from Brevio, you agree to the practices described in this policy.

If you have any questions about this policy, email <galiette@testing-orbit.com>.

## 1. What Brevio is

Brevio is an executive assistant in a closed beta. Today, Brevio watches your Gmail and sends you a short iMessage when something looks genuinely important. Every message in this beta is reviewed by the Brevio founder in a private Slack channel before it is sent. Brevio does not act on your behalf — it surfaces information so you can decide what to do.

## 2. Information we access

### 2.1 Google account (OAuth)

When you connect Google to Brevio, we request the following OAuth scope:

- `https://www.googleapis.com/auth/gmail.readonly` — read-only access to your Gmail messages.

Brevio uses this scope to fetch new messages from your Gmail inbox, identify a small subset as "important", and produce a one-sentence reason for each. Brevio does not send, draft, modify, label, archive, delete, or otherwise change anything in your Gmail.

**Calendar access is not currently requested.** A future version of Brevio is planned to request a separate, narrower scope (`https://www.googleapis.com/auth/calendar.events.readonly`) for read-only access to your **primary** Google Calendar within a rolling 48-hour window, reading only the event title, start time, and end time. Brevio will never request calendar write access. This policy will be updated to describe Calendar use **before** that scope is added to the consent screen, and the new scope will require its own consent from you. Until then, Brevio does not request or use any Calendar data.

### 2.2 Email content

Email subject lines, sender names, recipient addresses, message identifiers, and short metadata are read by Brevio. The body text of an email is also sent — only at the moment of ranking — to a third-party language model (OpenAI) so the model can classify whether the email is important and produce a one-sentence reason. The body is not stored after the classification call returns.

**What is stored:** sender address, subject, message identifier, ranker score, the one-sentence ranker reason, and operational metadata (timestamps, audit identifiers). These are stored to power the alert flow and the audit log.

**What is never stored:** the body text of your email, attachment names, attachment content, label structure beyond INBOX presence, or any other Gmail content beyond the fields listed above.

### 2.3 Mobile number

When you join the beta, you provide a mobile number so Brevio can send you an iMessage when an important email lands. We store a salted hash of your number for routing and a record of your STOP / START preference. We use a third-party messaging provider (SendBlue) to deliver the iMessage.

### 2.4 Operational data

For each Brevio interaction we record audit rows that include user identifiers, timestamps, action names, and structural counters (for example, the number of events in a calendar window, sanitized error reasons, kill-switch state). Audit rows are deliberately structured to **exclude** message bodies, calendar event content, subject text beyond what is needed to render an alert card, and any other free-form personal content.

## 3. How we use information

We use the information described above only to:

1. Identify Gmail messages that are likely to be important to you and produce a short reason for each.
2. Present those candidate alerts to the Brevio founder in a private Slack channel so the founder can approve or reject each alert before it is sent.
3. Deliver an approved alert to your phone as an iMessage.
4. Accept a STOP, START, or short feedback reply you send back, and adjust Brevio's behavior accordingly.
5. Operate, secure, debug, and improve the Brevio service (including the ranker, the alert renderer, and Brevio's learning systems described below).

**We do not use Gmail content, Calendar content, message content, or any user-provided content to train any machine learning model, ours or anyone else's.**

We do not sell user data to anyone, and we do not share user data with advertisers, brokers, or marketing partners.

## 4. Who we share information with

Brevio shares the minimum data necessary with the following service providers, only as needed to operate Brevio:

| Provider | What they receive | Why |
|---|---|---|
| **Google (Gmail API)** | Read-only Gmail requests using your OAuth token | To fetch the messages Brevio ranks. |
| **OpenAI** | Email body text and a structured prompt for the single classification call | To classify whether the email is important and produce a one-sentence reason. Brevio does not use OpenAI's training-eligible endpoints; OpenAI does not retain the content past the call under their standard API terms. |
| **SendBlue** | Your mobile number and the approved alert text | To deliver the iMessage to your phone. |
| **Slack** | Sender, subject, and the ranker's one-sentence reason for each candidate alert | To present the alert card to the Brevio founder for approval before any iMessage is sent. The body of your email is not posted to Slack. |
| **Database / infrastructure providers** | Audit rows, alert metadata, encrypted OAuth tokens, and other operational records | To host Brevio. Brevio currently uses Neon (Postgres) and Render. |

We do not share user data with any third party other than to operate Brevio as described in this table, except where required by law (see §10).

## 5. Data retention

- **OAuth tokens:** encrypted at rest with AES-256-GCM under a key Brevio controls; deleted when you revoke access or delete your Brevio account.
- **Alert metadata** (sender, subject, ranker reason, timestamps): retained for the duration of your participation in the Brevio beta so the assistant can learn from your reactions. Deleted on request and on account deletion.
- **Audit log:** retained while the service is operated, for security, debugging, and regulatory purposes. Audit rows are structural; they do not contain email bodies, calendar event content, or other free-form personal content.
- **Email body text:** never retained. Sent to the ranker at the moment of classification and discarded.
- **Calendar event content** (when Calendar is added in a future version): processed in memory only, for the duration of a single email-ranking decision, then discarded. Never written to a database, log, audit row, or any persistent storage.
- **Mobile number:** stored as a salted hash for the duration of your participation; deleted on request and on account deletion.

## 6. Security

- OAuth tokens are encrypted at rest under a key Brevio controls.
- OAuth flows use PKCE and HMAC-signed state with a single-use nonce to defend against replay.
- Inbound webhooks from SendBlue and Slack are verified with provider-specific signed-secret checks before any processing.
- Brevio operates behind a deny-by-default permission posture: outbound messaging is gated by per-user STOP state, provider-status, kill switches, and explicit founder approval. There is no automatic action on your behalf in the current beta.
- Errors surfaced into the audit log are passed through a sanitizer that converts provider error text into a fixed, non-personal set of reason codes before storage.

No system is perfectly secure. If you discover a security issue, email <galiette@testing-orbit.com>.

## 7. Your choices

- **Revoke Gmail access** at any time at <https://myaccount.google.com/permissions>. The next time Brevio attempts to read your inbox, the call will fail and Brevio will stop ranking your messages.
- **Stop receiving iMessages** by replying `STOP` to any Brevio message. Brevio will send one confirmation reply and then send nothing further. Reply `START` to re-enable.
- **Delete your Brevio account and data.** Email <galiette@testing-orbit.com>. Brevio will delete your OAuth tokens, your hashed mobile number, your alert metadata, and your feedback signals. Audit rows that reference only structural counters or sanitized error codes may be retained for security and debugging.
- **Object to a specific use.** Email <galiette@testing-orbit.com>. Because this is a closed beta with a small number of participants, requests are reviewed individually.

## 8. Beta status

Brevio is in a closed beta operated by a single founder with a small number of invited users. Behavior, scopes, retention rules, and Brevio's overall feature set will change as the beta progresses. Each change to scope or to data-handling will be reflected in this policy before it ships, and material changes will be communicated directly to active beta participants over iMessage or email.

## 9. Children

Brevio is not directed to children under 13, and Brevio does not knowingly collect information from anyone under 13. If you believe a child has connected an account, email <galiette@testing-orbit.com> and Brevio will delete the account and associated data.

## 10. Legal requests

Brevio will disclose user data only when required by a valid legal process or to protect the rights, safety, or property of Brevio, its users, or the public. Brevio is a small operation; we will narrow scope and challenge overbroad requests where reasonable.

## 11. International users

Brevio is operated from the United States and processes data on infrastructure located in the United States. If you connect a Google account from outside the United States, you consent to your data being transferred to and processed in the United States.

## 12. Changes to this policy

Brevio will update this policy when its data practices change. The "Last updated" date at the top reflects the most recent change. Material changes — for example, adding a new OAuth scope, adding a new data category, or expanding a retention period — will be reflected here **before** they take effect on the consent screen, and will be communicated to active beta participants.

## 13. Contact

Email: <galiette@testing-orbit.com>

---

### Operator footer (do not publish — strip before paste)

- Section 2.1 currently says Gmail readonly only and pre-announces Calendar. Update after v0.6.0C ships and Cloud Console actually requests the Calendar scope.
- Source for the Gmail section: [docs/privacy-copy-v0.5.md](../privacy-copy-v0.5.md) (friend-beta consent copy — keep aligned).
- Source for the Calendar pre-announcement language: [docs/v0.6.0-privacy-policy-calendar.md](../v0.6.0-privacy-policy-calendar.md). The published policy must accurately describe currently-requested scopes; pre-announcing a not-yet-requested scope is acceptable when framed as planned, not as active.
- After publishing, capture a screenshot of the live page and attach it to the v0.6.0B-F4 evidence row in [docs/v0.6.0B-oauth-scope-readiness.md §5](../v0.6.0B-oauth-scope-readiness.md#5-founder-fact-table-fill-in-as-f1f5-land).

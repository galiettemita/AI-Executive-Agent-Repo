# Brevio — Friend Beta Privacy Note (v0.5)

> **Draft 2026-05-29 for founder review.** This copy is rendered
> verbatim at the `/onboard` landing page bottom + reused on the
> Google consent screen for the friend's first connection. Update
> here, not in the route source — `apps/fomo/src/routes/onboard.ts`
> reads this file at startup.

---

## What Brevio does for you

Brevio reads your Gmail in the background, identifies messages
that look genuinely important (a real intro, an actual deadline,
something time-sensitive from a real person), and sends you a single
short iMessage when one lands. Founder reviews every alert in Slack
before any iMessage goes out — there is no auto-send in this beta.

## What Brevio reads

- Subject lines, sender names, and short metadata of your Gmail messages
- The body text of an email is sent to the ranker (an OpenAI model)
  to decide if the email is "important" or not. The model returns
  a one-sentence reason and a numeric score
- **Brevio does not store the body text of your email** in any
  database, log, audit row, or Slack post. The body content is
  read in-memory at rank time only, never persisted

## What the founder sees

- The founder reviews your important alerts in a Slack channel
- The founder card shows: sender, subject, and the one-sentence
  ranker reason
- **The founder does NOT see** the body of your email, attachment
  names, or any other content beyond what's listed above

## What goes to your phone

- A short iMessage from the Brevio number summarizing the email
- You can reply `STOP` from your phone at any time to stop all
  future alerts. `STOP` is recognized deterministically — no model
  decides whether you meant it

## What Brevio will never do (in this beta)

- Send any email on your behalf
- Auto-send any iMessage without founder review
- Share your data with any third party (other than the OpenAI
  ranker for the one classification call, which receives the email
  body for that single call and does not retain it per OpenAI's
  enterprise terms)
- Read messages outside your Gmail inbox
- Access your calendar, contacts, photos, or any other Google service

## What happens when you disconnect

- Texting `STOP` to the Brevio number disables future alerts for you
- Re-texting `START` re-enables them
- You can revoke Gmail access from Google's account settings at any
  time; the next time Brevio tries to poll, the read will fail and
  alerts stop

## Questions

This beta is one founder + a small number of friends helping each
other shape what "useful work assistant" actually means. Reply to
this iMessage thread or text the founder directly with any concern
about what Brevio is doing — they will answer personally.

---

By clicking "Connect with Google" below, you confirm:

- You read this note
- You consent to Brevio reading your Gmail messages with the
  `gmail.readonly` scope
- You understand the founder reviews every alert in Slack before
  any text goes to your phone
- You can text `STOP` at any time to disable Brevio

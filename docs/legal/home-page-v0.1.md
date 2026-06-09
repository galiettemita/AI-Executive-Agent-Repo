# Brevio — Home Page Content

> **Source of truth for the public app-info page.** Published as the homepage at <https://orbit-landing-ten.vercel.app/>. Listed as the **Application Home Page URL** on the Google OAuth consent screen.
>
> If the Vercel landing page already has a designed homepage, **do not replace it** — just confirm the elements in §A are present (app name, one-sentence description, beta status, support contact, links to Privacy + Terms). Add anything missing.
>
> If the Vercel landing page does not yet have substantive content, use §B as the full page copy.
>
> **Before publishing, reconcile the brand name** with the rest of your public surfaces (see the note at the top of [privacy-policy-v0.1.md](privacy-policy-v0.1.md)).
>
> **Drafted:** 2026-06-08.

---

## §A — Required elements (Google OAuth verification checklist)

Google's OAuth consent screen review checks that the Application Home Page URL hosts a page that:

1. clearly identifies the application by the same name shown on the consent screen,
2. describes what the application does in plain language,
3. links to (or surfaces) the Privacy Policy and Terms of Service,
4. provides a way to contact the application owner.

Confirm all four are present before submitting verification.

| Element | What it must say |
|---|---|
| Application name | The exact same name shown on the Google OAuth consent screen (e.g. "Brevio"). |
| One-sentence description | "Brevio is an executive assistant in beta that watches your Gmail and tells you only what's important." (or your own version that says the same thing). |
| Beta status | A visible line saying Brevio is in a closed beta. |
| Contact | The support email <galiette@testing-orbit.com> shown as plain text or a `mailto:` link. |
| Privacy link | Link to `/privacy` (the page published from [privacy-policy-v0.1.md](privacy-policy-v0.1.md)). |
| Terms link | Link to `/terms` (the page published from [terms-of-service-v0.1.md](terms-of-service-v0.1.md)). |

## §B — Drop-in homepage copy (only if the existing landing page is bare)

If the Vercel landing page is currently a placeholder, use this copy as the homepage content. Plain text — paste into your site's MDX, HTML, or rich-text editor as you prefer.

---

# Brevio

**Brevio is an executive assistant in beta.** It watches your Gmail and tells you only what's important.

You stay in control. Brevio surfaces information. Brevio does not send email on your behalf, does not write to your calendar, and does not act without your approval. Every alert in this beta is reviewed by Brevio's founder before it reaches your phone.

## How it works today

1. You connect your Gmail with read-only access.
2. Brevio reads new messages in your inbox and identifies a small subset as genuinely important.
3. The Brevio founder reviews each candidate alert in a private channel.
4. Approved alerts arrive as a short iMessage on your phone.
5. You can reply `STOP` at any time to stop all future alerts.

## What Brevio does not do

- Brevio does not send email for you.
- Brevio does not modify, label, archive, or delete anything in your Gmail.
- Brevio does not read your calendar today. (A read-only Calendar feature is planned; it will require its own consent and will be announced in the [Privacy Policy](/privacy) before it ships.)
- Brevio does not sell your data.
- Brevio does not use your email content to train AI models.

## Beta status

Brevio is in a closed beta with a small number of invited users. Features, behavior, and scope will change as the beta progresses.

## Contact

Questions: <galiette@testing-orbit.com>

## Legal

- [Privacy Policy](/privacy)
- [Terms of Service](/terms)

---

### Operator footer (do not publish — strip before paste)

- The Google OAuth consent screen requires the Application Home Page URL to match a page that "clearly describes" the application. A landing page selling an unrelated product, or a page without the application name, can be grounds for verification rejection.
- If the Vercel landing page currently presents under a different brand name (e.g. "Orbit"), Google reviewers may flag the mismatch between the consent screen ("Brevio") and the homepage ("Orbit"). Reconcile before submitting verification.
- Consent-screen field reference: **Application home page**, **Application privacy policy link**, **Application terms of service link** — three distinct URL fields, populated with the three URLs above.

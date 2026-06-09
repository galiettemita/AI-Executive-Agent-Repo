# Publishing Brevio's legal pages — operator runbook

**Phase:** v0.6.0B-F4 (legal pages). **Tier:** TIER 3 (static markdown for founder to publish elsewhere; no runtime code in this repo).

This runbook walks the founder through publishing the three public pages drafted in this directory so that fact **F4** in [docs/v0.6.0B-oauth-scope-readiness.md §5](../v0.6.0B-oauth-scope-readiness.md#5-founder-fact-table-fill-in-as-f1f5-land) can be marked complete.

## Pages to publish

| Source file (this repo) | Target URL | Google Console field |
|---|---|---|
| [home-page-v0.1.md](home-page-v0.1.md) | <https://orbit-landing-ten.vercel.app/> | Application home page |
| [privacy-policy-v0.1.md](privacy-policy-v0.1.md) | <https://orbit-landing-ten.vercel.app/privacy> | Application privacy policy link |
| [terms-of-service-v0.1.md](terms-of-service-v0.1.md) | <https://orbit-landing-ten.vercel.app/terms> | Application terms of service link |

## 0. Brand reconciliation (do this first)

This repo says "Brevio". The Vercel project is `orbit-landing-ten`. Your support email is `galiette@testing-orbit.com`. The drafts all say "Brevio". Before publishing, decide one of:

- **Keep Brevio everywhere.** Rename the Vercel project (Vercel → project settings → general → project name), update homepage copy to say "Brevio". The Google OAuth consent screen application name must also be "Brevio".
- **Switch the public name to Orbit (or another name).** Find/replace "Brevio" → "<new name>" in all three drafts before publishing. Update the Google OAuth consent screen application name to match.

**Do not publish with mismatched names.** A Google verification reviewer can reject the submission for "the application name on the homepage does not match the application name on the consent screen."

## 1. Pre-publish checklist (per page)

For each of the three pages:

- [ ] Find/replace `<publish date>` in the "Last updated" line with today's date in `YYYY-MM-DD` format.
- [ ] Strip the operator footer block at the bottom of the file (everything after `### Operator footer (do not publish — strip before paste)`).
- [ ] Strip the top blockquote (everything between the first `> ` and the first `---`).
- [ ] In `terms-of-service-v0.1.md` §11, replace the `<jurisdiction>` placeholders with the actual governing-law state/county.
- [ ] Confirm `<galiette@testing-orbit.com>` is the email you want shown publicly. If you set up a domain-aligned alias (e.g. `support@brevio.app`), swap it in all three pages.

## 2. Publish on Vercel

The Vercel landing site is the publish target. The flow depends on what framework `orbit-landing-ten` uses; the steps below cover the common cases.

### 2.1 If `orbit-landing-ten` is a Next.js App Router project

1. Pull the Vercel project repo locally.
2. Create `app/privacy/page.tsx` and `app/terms/page.tsx`. Each renders the corresponding markdown. If the project already has a markdown rendering pipeline (e.g. `next-mdx-remote`, `contentlayer`), reuse it. If not, the simplest path is to convert the markdown to JSX/HTML by hand or use a one-shot converter.
3. Update `app/page.tsx` to either match §A of [home-page-v0.1.md](home-page-v0.1.md) or replace its content with §B if the existing homepage is bare.
4. Add a footer to the layout with `<Link href="/privacy">Privacy</Link>` and `<Link href="/terms">Terms</Link>` so the pages are discoverable.
5. Commit, push, let Vercel deploy.
6. Confirm both URLs return 200 in an incognito window: `/privacy` and `/terms`.

### 2.2 If `orbit-landing-ten` is a plain HTML / static site

1. Convert each markdown file to HTML using your preferred renderer (e.g. `pandoc -f markdown -t html5 -s privacy-policy-v0.1.md -o privacy.html`).
2. Wrap each in your existing site's layout/CSS so the brand reads consistently.
3. Deploy `privacy.html` to `/privacy` and `terms.html` to `/terms`.
4. Update homepage `index.html` per §A.
5. Add Privacy + Terms links to the site footer.

### 2.3 If `orbit-landing-ten` uses an MDX-based generator (e.g. Docusaurus, Astro, Nextra)

1. Drop the markdown files directly into the content directory under names matching the route.
2. Update the homepage source per §A.
3. Add nav/footer entries.
4. Deploy.

### 2.4 Verify

- [ ] `curl -sI https://orbit-landing-ten.vercel.app/privacy | head -1` returns `HTTP/2 200`.
- [ ] `curl -sI https://orbit-landing-ten.vercel.app/terms | head -1` returns `HTTP/2 200`.
- [ ] Open both URLs in an incognito window. Confirm the page renders without auth and the content matches the markdown source.
- [ ] Confirm the homepage at <https://orbit-landing-ten.vercel.app/> identifies "Brevio" (or your chosen public name) and links to Privacy + Terms.

## 3. Update the Google OAuth consent screen

Founder action — Claude cannot reach Google Cloud Console.

1. Open <https://console.cloud.google.com/apis/credentials/consent> for the Brevio Google Cloud project.
2. Click **Edit App**.
3. Fill in (or update) the three URL fields:
   - **Application home page** → `https://orbit-landing-ten.vercel.app/`
   - **Application privacy policy link** → `https://orbit-landing-ten.vercel.app/privacy`
   - **Application terms of service link** → `https://orbit-landing-ten.vercel.app/terms`
4. Confirm the **App name** matches the brand name on the published homepage (the v0.5 verify-warning Friend B saw was driven by mismatched/empty consent-screen metadata — see [[v05-4-pass]]).
5. Confirm the **Authorized domains** list includes `vercel.app` (or your custom domain when you migrate off).
6. Save.
7. Screenshot the saved state. Attach the screenshot to the v0.6.0B-F4 evidence row in [docs/v0.6.0B-oauth-scope-readiness.md §5](../v0.6.0B-oauth-scope-readiness.md#5-founder-fact-table-fill-in-as-f1f5-land) along with the publish date.

## 4. What this phase does NOT change

This phase is TIER 3. The following are explicitly out of scope and unchanged:

- No runtime Brevio behavior changes.
- No backend route added or modified in `apps/fomo/`.
- No env var changes.
- No OAuth scope changes (Brevio still requests `gmail.readonly` only).
- No Calendar API access (`calendar.events.readonly` is **pre-announced** in the published privacy policy but **not** requested on the consent screen; that addition is gated behind v0.6.0B fact F5 and the v0.6.0C scope-add work, both still locked).
- No Calendar runtime code.
- No Tool Gateway.
- No browser automation.
- No Founder Command Surface.
- No new audit kinds.
- No new memory_signal kinds.
- No new DB migration.
- No Friend C expansion.
- No SendBlue work.

If publishing these pages surfaces a new question — for example, the homepage needs branding work or the Vercel project structure isn't suitable for adding routes — capture it as a v0.6.0B-F4 follow-up. Do **not** quietly expand scope.

## 5. After publishing

1. Mark F4 in [docs/v0.6.0B-oauth-scope-readiness.md §5](../v0.6.0B-oauth-scope-readiness.md#5-founder-fact-table-fill-in-as-f1f5-land) with the live URL + publish date.
2. Re-pulse Claude. The remaining v0.6.0B founder facts (F1, F2, F3, F5) can land in any order; F4 was the only one blocked on publishing legal pages.
3. **Do not start v0.6.0C until F1-F5 are all answered.** That gate is locked.

## 6. Confirmation

I confirm:

- Drafting these markdown files does not authorize Calendar scope.
- Drafting these markdown files does not add Calendar runtime code.
- Drafting these markdown files does not modify `apps/fomo/` runtime.
- Drafting these markdown files does not change Google Cloud Console (founder action only, after publish).
- The privacy policy pre-announces Calendar use as planned-not-active language; the actual consent-screen scope addition is still gated behind v0.6.0B F5 and v0.6.0C.

## 7. Reference links

- [[v05-15-pass]] — most recent shipped phase; pivot to non-FOMO work per [[end-of-fomo-hardening-pivot]].
- [[end-of-fomo-hardening-pivot]] — the lock that put Brevio on the Calendar-readiness lane.
- [[risk-tiered-verification]] — tier rule that classifies this phase TIER 3.
- [docs/v0.6.0B-oauth-scope-readiness.md](../v0.6.0B-oauth-scope-readiness.md) — parent v0.6.0B checklist; F4 row updated when these pages go live.
- [docs/v0.6.0-privacy-policy-calendar.md](../v0.6.0-privacy-policy-calendar.md) — earlier-drafted Calendar wording (already absorbed into [privacy-policy-v0.1.md](privacy-policy-v0.1.md) §2.1).
- [docs/privacy-copy-v0.5.md](../privacy-copy-v0.5.md) — friend-beta consent copy. Keep aligned when the public privacy policy changes.

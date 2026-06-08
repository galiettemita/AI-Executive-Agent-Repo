# Provider Production Readiness — Umbrella Tracker

Skimmable in 60s. Detail lives in the per-track docs.

This phase is **docs and ops only**. No runtime code. No new tools. No new providers. No founder command surface scoping. No SendBlue purchase recommendation as confirmed. No Google scope changes.

Founder rule (permanent): build production-grade paths, not crutches. Default to the fastest safe proof, not the maximum possible proof. See [[risk-tiered-verification]].

---

## TL;DR (60 seconds)

- **Current decision.** Start two external clocks in parallel (Google OAuth verification submission + SendBlue support/dashboard fact-gathering). No purchase, no scope change, no runtime work this phase.
- **What founder must do next.** (1) Cloud Console: confirm consent screen completeness + add briefed-friend emails to External Test Users (immediate per-email unblock); decide whether to submit for CASA. (2) SendBlue: open dashboard to confirm current plan + contact cap, file one support ticket covering the 14 SendBlue TO-VERIFY items (plan features, AI Agent feature claims, pricing, migration steps, webhook SLA, OPTED_OUT clearance).
- **What Claude can do next.** Update these docs as verified facts come back; draft (separately) the next-phase scope memo for Hardening backlog #3 if/when founder opens that gate; grep-confirm any code citation drift. No runtime code, no new tools, no new audit kinds.
- **What is blocked until provider facts are verified.** SendBlue plan upgrade decision; CASA submission go/no-go; Friend C onboarding; 5–10 user beta expansion; honest revision of any AI Agent feature claim. All 29 TO-VERIFY items are listed across the two detail docs.
- **What NOT to build yet.** Founder Command Surface (hosted webhook for "text Brevio anytime"); local sandbox / Tone Gallery / CLI simulator; per-user observability dashboard; periodic SendBlue reconciliation worker; runtime kill-switch mutation API; self-serve invite flow; any scope or auth runtime change.

---

## 1. Phase summary

**What this phase is.** Bring Brevio's two live external providers (Google OAuth, SendBlue) to a state where a non-founder user can complete the onboarding chain without alarming interstitials, silent webhook drops, or opaque error states — and where the operator (founder) can diagnose provider-side failures from `audit_log` alone.

**What this phase is NOT.** Not a runtime change. Not a smoke. Not a beta expansion (real-friend cap remains 3 per [[three-friend-beta-cap]]). Not a SendBlue plan upgrade decision — that decision is gated on dashboard facts only the founder can read.

**Tier classification (founder-locked).**

| Track | Tier | Why |
|---|---|---|
| Google OAuth | TIER 1 | Provider/OAuth external-clock trigger (verification submission → Google-side review SLA). Docs/ops only for this workflow — runtime untouched. |
| SendBlue | TIER 2 | Provider-confirmation track. Docs/ops only; promotes to TIER 1 if any runtime behavior changes. |
| Umbrella tracker (this doc) | TIER 2/3 | Coordination + docs. |
| Hardening backlog #3 | (separate phase, future) | Sanitized `error_code` + `error_reason` on Gmail/dispatch audits. See [[hardening-backlog]] item #3. |

See [[risk-tiered-verification]] for tier triggers and ceremony depth rules. See [[real-or-absent-no-half-wired]] for the no-half-wired discipline that gates any runtime promotion.

---

## 2. Status dashboard

| Track | Status | Tier | Owner | External clock | Next decision |
|---|---|---|---|---|---|
| Google OAuth | Docs drafted; submission packet TO ASSEMBLE | TIER 1 | Founder (dashboard + submission); Claude (docs only) | Google review SLA TO VERIFY (founder/dashboard) once submitted | Founder reviews submission packet → submits in Google Cloud Console |
| SendBlue | Plan/tier facts TO VERIFY in dashboard; runtime semantics confirmed from code | TIER 2 | Founder (dashboard + support); Claude (docs only) | SendBlue support response time TO VERIFY (founder/support) | Founder pulls current plan name + contact cap from dashboard → decide if upgrade is the right next move |
| Hardening backlog #3 (sanitized error fields) | Queued; not in scope this workflow | (own 6Q gate later) | Founder (gate decision); Claude (proposal) | None | Defer until founder opens its own gate; see [[hardening-backlog]] |

Detail per track lives in the linked docs in section 3.

---

## 3. Three-track view

### 3.1 Google OAuth → [Provider Readiness — Google OAuth](provider-readiness-google-oauth.md)

**Known from code/repo.**
- Scope = `gmail.readonly` only. Defined at [apps/fomo/src/adapters/gmail/client.ts:25](../apps/fomo/src/adapters/gmail/client.ts) (`GMAIL_READONLY_SCOPE`) and consumed at [apps/fomo/src/routes/oauth-google.ts:229](../apps/fomo/src/routes/oauth-google.ts).
- PKCE + HMAC-signed state + single-use nonce: [apps/fomo/src/routes/oauth-google.ts](../apps/fomo/src/routes/oauth-google.ts) (`generatePKCEVerifier`, `buildState`, `verifyState`, nonce row consumption).
- `POST /oauth/google/start` is session-authenticated and returns `authorize_url`. The callback `GET /oauth/google/callback` is the only Google-facing route. See [[brevio-oauth-google-reauth-procedure]] for the exact 5-step session-mint → curl → browser → callback → verify sequence.
- Encrypted token store + `oauth_tokens.needs_reauth` flag.
- Env vars: `BREVIO_OAUTH_REDIRECT_URI_GOOGLE`, `BREVIO_SESSION_SIGNING_KEY` (per [[brevio-oauth-google-reauth-procedure]]).

**Known from founder smoke.**
- v0.5.4 Friend B (Sheila) called the "this app could be unsafe" interstitial "alarming" — see [[v05-4-pass]] candidate #1. Brevio OAuth app is in Testing mode.

**TO VERIFY (founder / Google Cloud Console).**
- Current app publishing status (Testing vs In production).
- OAuth consent screen completeness: app name, user-facing email, app logo, app domain, privacy policy URL, terms of service URL, authorized domains, developer contact email.
- External Test Users list contents (must include every onboarded friend's Google email while in Testing).
- Redirect URIs as registered in Cloud Console (must match `BREVIO_OAUTH_REDIRECT_URI_GOOGLE` exactly).
- Any prior CASA submission status (CASA is required for restricted scopes; `gmail.readonly` IS a restricted scope per Google's published list — TO VERIFY current Google policy).
- Current quota / user count from Cloud Console.
- Google verification review SLA (typical multi-week, exact TO VERIFY from Google's current published timelines).

### 3.2 SendBlue → [Provider Readiness — SendBlue](provider-readiness-sendblue.md)

**Known from code/repo.**
- 3-outcome semantics (registered / not_registered / transient_fail) at [apps/fomo/src/adapters/sendblue/client.ts:395](../apps/fomo/src/adapters/sendblue/client.ts) (`registerContact`).
- Webhook auth = header-equality timing-safe comparison of a SHARED SECRET against the `sb-signing-secret` header (overridable via `SENDBLUE_WEBHOOK_SECRET_HEADER`). NOT HMAC over the body. Documented in source at [apps/fomo/src/adapters/sendblue/client.ts:612-682](../apps/fomo/src/adapters/sendblue/client.ts) and [apps/fomo/src/routes/sendblue-inbound.ts](../apps/fomo/src/routes/sendblue-inbound.ts).
- `OPTED_OUT` error extraction at [apps/fomo/src/adapters/sendblue/client.ts:104-160](../apps/fomo/src/adapters/sendblue/client.ts) (uppercased machine-readable token; feeds the v0.5.3 drift detector).
- Reconciliation script at [apps/fomo/src/adapters/sendblue/reconcile.ts](../apps/fomo/src/adapters/sendblue/reconcile.ts) — paginates `/api/v2/messages`, diffs against `audit_log`, audits gaps as `fomo.sendblue.delivery_gap_detected`. See [[v05-3-pass]] item #4.
- Env var names: `SENDBLUE_API_KEY_ID`, `SENDBLUE_API_SECRET_KEY`, `SENDBLUE_WEBHOOK_SECRET`, `SENDBLUE_WEBHOOK_SECRET_HEADER`, `FOMO_SEND_ENABLED`, `FOMO_SENDBLUE_INBOUND_ENABLED`, `FOMO_FOUNDER_PHONE_NUMBER`. Referenced from [apps/fomo/src/index.ts](../apps/fomo/src/index.ts).
- Kill switch: `sendblue_inbound_enabled` (route unmounts when false) at [apps/fomo/src/routes/sendblue-inbound.ts:343](../apps/fomo/src/routes/sendblue-inbound.ts).

**Known from founder smoke.**
- v0.5.2 Morris webhook delivery gap (server-crash window 11h; SendBlue retries exhausted; reconcile script later detected the exact incident). See [[v05-2-pass]] §6 caveat.
- Founder phone `OPTED_OUT` state accumulated across phases; drift detector fired live in v0.5.6. See [[v05-3-pass]] and [[v05-6-pass]].
- v0.5.4 cross-tenant isolation PASS (Sheila row added, Morris/founder/gm3258 rows untouched). See [[v05-4-pass]].

**TO VERIFY (founder / SendBlue dashboard or support).**
- Current plan name in dashboard.
- Current contact cap (count of verified contacts allowed).
- Exact pricing of the next tier up from current.
- AI Agent tier feature claims (specifically: does it remove the "contact must text first" gate, does it expose a carrier-level opt-out webhook, inbound message cap per day). PREVIOUSLY MEMOIZED in [[sendblue-plan-gates]] as ~$100/mo/line and 1000 inbound/day — those numbers are now ~7 days old, treat as PLACEHOLDER until founder reconfirms in dashboard.
- Inbound webhook delivery SLA / retry policy.
- Support response time SLA.
- `OPTED_OUT` clearance procedure (carrier-side; SendBlue support may need to file with carrier).
- Whether any "verified contact" approval flow for new numbers exists beyond inbound-first.

### 3.3 Hardening backlog #3 — sanitized `error_code` + `error_reason`

Separate but related. NOT in scope this workflow. Tracked in [[hardening-backlog]] item #3.

Real-incident evidence: v0.5.8 smoke 2026-06-06, Q1.A wire-format bug ran silently for ~10 cycles because per-user Gmail error context never reached `audit_log`. Direct relevance to this phase: without #3, the next Google or SendBlue provider-side regression hides the same way. Founder-flagged as critical, but its own 6Q gate.

---

## 4. Cross-cutting actions

### Do now

| Action | Owner | Notes |
|---|---|---|
| Draft Google OAuth submission packet (consent screen content, scope justification, privacy policy URL placeholder, demo video script) | Claude (docs) | Output is markdown in [provider-readiness-google-oauth.md](provider-readiness-google-oauth.md); founder uploads/submits in Cloud Console. |
| Document SendBlue current-state checklist (plan, contact cap, support ticket templates) | Claude (docs) | Output is markdown in [provider-readiness-sendblue.md](provider-readiness-sendblue.md). |
| Pull current plan name + contact count from SendBlue dashboard | Founder | Required input to the SendBlue decision in section 5. |
| Pull OAuth consent screen status + Test Users list from Google Cloud Console | Founder | Required input to the Google submission decision in section 5. |
| Confirm `OPTED_OUT` state on founder phone is still present (vs cleared) | Founder | Needed to scope any future smoke involving founder-phone outbound. See [[v05-6-pass]] for v0.5.3 drift detector behavior. |

### Wait until trigger

| Action | Trigger |
|---|---|
| Submit Google verification | Submission packet reviewed + founder approval. |
| Decide whether to upgrade SendBlue plan | Founder has confirmed current plan name + contact cap + has tested AI Agent tier claims via support if needed. |
| Run any provider smoke | Both tracks are at a known-state baseline AND a runtime change exists that warrants smoking. NOT before. |
| Promote hardening backlog #3 (sanitized error fields) | Founder opens its own 6Q gate. |

### Skip / no crutch

Per founder rule "production-grade paths, not crutches":

- NO local crutch sandbox that mocks Google verification.
- NO dashboard proposal (no Brevio-side UI; founder uses Google Cloud Console + SendBlue dashboard directly).
- NO Friend C expansion as a way to "prove" provider readiness. Real-friend cap is 3; current count is 2 (Morris ✓, Sheila ✓). See [[three-friend-beta-cap]].
- NO auto-send. Permission Gate stays approve-each.
- NO new tools / executors.
- NO new providers added to the surface area while these two are mid-readiness.
- NO scope expansion to `gmail.modify` / `gmail.send` (would re-trigger Google verification).
- NO premature SendBlue plan upgrade purchase — wait until founder has confirmed the upgrade actually removes the specific blocker (the AI Agent tier claims are TO VERIFY).

---

## 5. Decision triggers

A track advances ONLY when its named trigger fires.

| Track | Advances to "submit" / "upgrade" when |
|---|---|
| Google OAuth | Submission packet is complete; founder has confirmed consent screen + Test Users + redirect URIs in Cloud Console; founder explicitly approves submission. |
| SendBlue | Founder has confirmed current plan + cap in dashboard; has either (a) opened a support ticket to ask the AI Agent tier questions OR (b) accepted the documented unknowns; has explicitly decided "upgrade now" vs "stay on current plan and re-evaluate at trigger X". |
| Hardening backlog #3 | Founder opens its own 6Q gate. Not auto-promoted by this workflow. |

**Anti-trigger (do NOT advance):**
- "Submit Google verification because we're stuck waiting." Submission is a multi-week external clock; submit only when the packet is ready.
- "Upgrade SendBlue because Friend C is coming." Friend C is not scheduled and is optional (see [[three-friend-beta-cap]]).
- "Run a provider smoke to validate the docs." Smokes prove reality unit tests can't reach. Docs don't need smokes (see [[risk-tiered-verification]] "do not use smoke tests as a ritual").

---

## 6. What NOT to build yet (founder-locked list)

- NO runtime code in this workflow.
- NO SendBlue plan upgrade purchase as if confirmed beneficial.
- NO Google scope changes (`gmail.modify`, `gmail.send`, calendar scopes, etc.).
- NO Founder Command Surface scoping (separate future phase).
- NO local crutch sandbox / verification mock.
- NO dashboard / admin UI proposal.
- NO Friend C expansion.
- NO auto-send.
- NO new tools / executors.
- NO new external providers.
- NO L2+ surface activation (calendar, drafting, sending, MCP, browser, autonomy tiers).

---

## 7. Open questions for founder (max 3)

1. **SendBlue track destination.** Is the goal "stay on current plan + document the limits" or "upgrade to remove the inbound-first gate"? Drives whether section 3.2 ends with a checklist or a purchase recommendation.
2. **Google submission timing.** Do you want to submit verification now (multi-week external clock starts) or hold until a specific milestone (e.g., before any v1.0 friend onboarding)? The "alarming interstitial" stays in place until verified.
3. **Privacy policy URL.** Does Brevio have a published privacy policy URL we can cite in the OAuth consent screen, or is that an additional asset to produce as part of this phase?

---

## 8. Owner-only actions (founder dashboard/support steps Claude CANNOT do)

Claude has no access to: the SendBlue dashboard, the SendBlue support portal, the Google Cloud Console, the founder's email, carrier-level data, or the founder's phone. The following actions can only happen on the founder's side:

**Google Cloud Console:**
- Read app publishing status (Testing vs In production).
- Read / edit OAuth consent screen content.
- Read / edit External Test Users list.
- Read / edit Authorized redirect URIs.
- Submit verification (with the assembled packet from this workflow).
- Read prior CASA submission status / artifacts.
- Read current quota and user count.

**SendBlue dashboard:**
- Read current plan name and pricing.
- Read current verified-contact list and cap.
- Read inbound webhook delivery logs.
- Open a support ticket (verify AI Agent tier feature claims; ask about `OPTED_OUT` carrier clearance; ask about inbound webhook SLA).
- Trigger contact verification on the founder side (e.g., re-text from a phone that previously OPTED_OUT, where carrier allows).

**Founder's phone:**
- Confirm whether `OPTED_OUT` is still set on the founder's phone (text START; observe whether reply lands).
- Re-text the SendBlue sender number from any new phone needed for testing.

**Founder's email:**
- Receive any verification correspondence from Google.
- Receive any support replies from SendBlue.

When this workflow produces a checklist, the founder is the one who runs it.

---

## References

- [[risk-tiered-verification]] — tier classification rules; default to fastest safe proof.
- [[real-or-absent-no-half-wired]] — no half-wired features; promotion to runtime is gated.
- [[no-gate-creep-on-extra-smokes]] — one smoke per phase; do not add validation rituals retroactively.
- [[surface-pr-merge-readiness]] — proactively surface mergeable PRs.
- [[three-friend-beta-cap]] — real-friend cap is 3 upper bound; Friend C optional.
- [[brevio-oauth-google-reauth-procedure]] — exact 5-step OAuth re-auth procedure; do not guess.
- [[sendblue-plan-gates]] — ~7-day-old SendBlue tier facts; treat as PLACEHOLDER until founder reconfirms.
- [[v05-2-pass]] — v0.5.2 Morris webhook gap incident.
- [[v05-3-pass]] — v0.5.3 hardening (auto-register, OAuth auto-refresh, pg pool, reconcile script).
- [[v05-4-pass]] — v0.5.4 Sheila smoke + 3 product findings including OAuth interstitial.
- [[v05-6-pass]] — v0.5.6 OPTED_OUT drift detector firing live.
- [[hardening-backlog]] — item #3 (sanitized `error_code` + `error_reason`).

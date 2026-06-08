# Provider Production Readiness — Google OAuth

**Tier:** 1 (provider/OAuth external-clock trigger)
**Workflow scope:** docs/ops only — no runtime code.
**Owner of external state:** Founder (Google Cloud Console). Claude has no Console access.

This doc is the single source of truth for what Brevio's Google OAuth integration looks like today, what is blocked on external Google verification, and what the founder must do in Cloud Console to unblock cohort expansion. Every "what's in Console" claim is marked `TO VERIFY (founder/Cloud Console)` because Claude cannot read Console state.

---

## 1. Current known state

### 1.1 Known from code/repo

All facts in this section are grep-confirmed against the working tree at HEAD of `phase-v0.5.12-live-ranker-pil-guarded`. File paths are clickable below.

**Provider identity and endpoints**

- Brevio supports exactly one OAuth provider in v0.1: `'google'`. There is no Microsoft/Outlook/iCloud client. See `SUPPORTED_PROVIDERS: OAuthProviderId[] = ['google']` in [providers/index.ts](apps/fomo/src/security/oauth/providers/index.ts).
- Authorize URL: `https://accounts.google.com/o/oauth2/v2/auth`
- Token URL: `https://oauth2.googleapis.com/token`
- Revoke URL: `https://oauth2.googleapis.com/revoke`
- Extra authorize params hardcoded: `access_type=offline`, `prompt=consent` (forces refresh-token issuance and re-consent screen on every grant).

**OAuth scope**

- Exactly one scope is requested: `https://www.googleapis.com/auth/gmail.readonly`. Constant `GMAIL_READONLY_SCOPE` defined in [adapters/gmail/client.ts](apps/fomo/src/adapters/gmail/client.ts). No `gmail.send`, no `gmail.modify`, no `gmail.labels`, no `gmail.metadata`, no `https://mail.google.com/`. This is a *Google-classified restricted scope* (see §5 for verification implications).
- Comment at top of `client.ts`: "FOMO_PLAN §9.1 + §9.2: v0.1 uses `gmail.readonly` scope only."

**Env vars (required to enable routes)**

| Env var                              | Purpose                                         | Source                                                      |
| ------------------------------------ | ----------------------------------------------- | ----------------------------------------------------------- |
| `GOOGLE_CLIENT_ID`                   | OAuth client ID from Cloud Console              | [providers/index.ts](apps/fomo/src/security/oauth/providers/index.ts) line 40 |
| `GOOGLE_CLIENT_SECRET`               | OAuth client secret from Cloud Console          | line 41                                                     |
| `BREVIO_OAUTH_REDIRECT_URI_GOOGLE`   | Redirect URI Cloud Console must allow-list       | line 42                                                     |
| `BREVIO_TOKEN_KEK`                   | KEK for AES-256-GCM token-at-rest encryption    | [token-crypto.ts](apps/fomo/src/security/token-crypto.ts) line 29 |

If any of `GOOGLE_CLIENT_ID` / `GOOGLE_CLIENT_SECRET` / `BREVIO_OAUTH_REDIRECT_URI_GOOGLE` are missing, `/oauth/google/*` returns a "not configured" disabled response — see [index.ts](apps/fomo/src/index.ts) line 148. The token KEK has a stricter rule: production refuses to boot without it unless `BREVIO_DEV_MODE=true` (per-process random KEK, dev-only).

**Routes**

- `POST /oauth/google/start` — session-middleware-authenticated. Generates PKCE verifier + S256 challenge, builds HMAC-signed state, stores in-memory nonce row, returns `{ authorize_url, state, nonce }`. See [routes/oauth-google.ts](apps/fomo/src/routes/oauth-google.ts) lines 91–129 and 333.
- `GET /oauth/google/callback` — unauthenticated route (Google's redirect; trust derives from HMAC-signed state). Verifies state, consumes nonce, exchanges code for tokens, encrypts tokens at rest, seeds Gmail history cursor. Lines 131–290, 351.

**Cryptographic posture**

- State parameter: HMAC-signed claims. `buildState` / `verifyState` use timing-safe comparison and enforce TTL. See [oauth/state.ts](apps/fomo/src/security/oauth/state.ts).
- PKCE: S256 challenge with 32-byte random verifier (`PKCE_VERIFIER_BYTES = 32`). Verifier sent with code-for-token exchange — see [oauth/exchange.ts](apps/fomo/src/security/oauth/exchange.ts) line 102.
- Nonce: single-use, in-memory, TTL-enforced. Mutated to `consumed: true` on first callback.
- Tokens at rest: AES-256-GCM with on-disk layout `nonce(12) || ciphertext || tag(16)`, AAD = `userId || provider || key_version`. Layered store in [token-store.ts](apps/fomo/src/security/oauth/token-store.ts). Migration 012 reserves `key_version` for future KEK rotation.
- Refresh tokens: encrypted-at-rest separately from access tokens; nullable in store.

**Onboard variant**

- `/onboard` (the v0.5.1+ multi-tenant invite flow) uses the same OAuth state shape and PKCE substrate ([onboard.ts](apps/fomo/src/routes/onboard.ts) lines 348–409), so all verification status considerations apply identically to onboarding new tenants.

### 1.2 Known from founder smoke

- Friend B's onboarding hit Google's "this app could be unsafe" / "unverified app" interstitial. Founder logged it as a future-cohort blocker in [[v05-4-pass]] §findings: *"Brevio OAuth app is in Testing mode; Google shows a 'this app could be unsafe' interstitial. Future-cohort blocker. Fix path: submit for Google verification (CASA assessment for restricted scopes — multi-week process). Until then, briefing script MUST warn friends about the warning."*
- 4 distinct Google accounts have successfully completed the full grant flow against the production client: founder (techsmarterusa), Morris, gm3258, Sheila — per phase-pass entries [[v05-2-pass]], [[v05-3-pass]], [[v05-4-pass]]. This is empirical evidence the code path works; it is *not* evidence of remaining quota.
- The session-mint → curl → browser → callback sequence used by the founder for re-auth is documented in [[brevio-oauth-google-reauth-procedure]] and matches the route shape in §1.1 exactly.

### 1.3 TO VERIFY (founder / Google Cloud Console)

Claude cannot see any of the following. These are blocker-level inputs the founder must read directly off the Cloud Console.

| Item                                                          | Why it matters                                                                   |
| ------------------------------------------------------------- | -------------------------------------------------------------------------------- |
| Publishing status (`Testing` vs `In production`)               | Testing mode caps at 100 distinct test users *(TO VERIFY — Google has historically published this figure; confirm current Console wording)* and shows the unverified-app warning. |
| If `Testing`: External Test Users list contents and count      | Each friend's exact Google account must be in this list before they can grant.   |
| OAuth consent screen: User Type (`External` vs `Internal`)     | Determines whether Workspace-only restriction applies.                            |
| App name shown on consent screen                                | This is what the friend sees as "Brevio wants to access your account."           |
| App home page URL                                               | Required field for verification.                                                  |
| App privacy policy URL                                          | Required field for verification of restricted scopes.                             |
| App Terms of Service URL                                        | Required for verification when applicable.                                        |
| Authorized domains list                                         | Must match the eTLD+1 of the privacy/home URLs *and* the redirect URI.            |
| Developer contact email / support email                         | Must be monitored — Google sends verification correspondence there.               |
| Registered redirect URIs (exact strings)                        | Must include the production value of `BREVIO_OAUTH_REDIRECT_URI_GOOGLE`. Mismatch = `redirect_uri_mismatch` at callback. |
| App logo (uploaded? branded?)                                   | Required for verification; reduces consent-screen friction.                       |
| Scopes declared on the consent screen                           | Must match exactly what code requests (`gmail.readonly`). Any drift causes Google to re-flag for re-verification. |
| Prior CASA submission status                                    | If a CASA assessment was started, vendor, tier, current step.                     |
| Current "users" gauge on the Cloud Console OAuth dashboard      | Current count toward the test-user cap.                                           |
| Brand verification status (separate from OAuth verification)    | Brand verification is a Google-side process distinct from OAuth verification.     |

---

## 2. Unknowns

Restated explicitly so the founder can walk Console once and tick the list:

1. Publishing status (Testing / In production).
2. External Test Users list — exact emails.
3. User Type (External / Internal).
4. App name, logo, home URL, privacy policy URL, ToS URL, support email, developer contact email.
5. Authorized domains.
6. Registered redirect URIs (exact case-sensitive strings).
7. Whether CASA has been initiated and with which assessor.
8. Current OAuth user gauge.
9. Whether brand verification is started/complete.
10. Whether any non-`gmail.readonly` scopes are declared on the consent screen (drift between Console and code).

---

## 3. Classification

| # | Item                                                                              | Class    | Notes                                                                |
| - | --------------------------------------------------------------------------------- | -------- | -------------------------------------------------------------------- |
| 1 | App in `Testing` mode + showing "unverified app" interstitial                      | Friction | Real per [[v05-4-pass]]. Briefing script mitigates today.            |
| 2 | Friend's Google account NOT on External Test Users list                             | Blocker  | Friend literally cannot grant. Immediate fix: add their exact email. |
| 3 | Missing/invalid privacy policy URL                                                 | Blocker  | Blocks CASA verification submission for restricted scopes.           |
| 4 | Missing/invalid Terms URL                                                          | Friction | Sometimes required; assessor-dependent.                              |
| 5 | Authorized domains drift vs redirect URI / privacy URL                              | Blocker  | Causes `redirect_uri_mismatch` and verification rejection.           |
| 6 | Console-declared scopes drift from code-requested scope                             | Blocker  | Google flags as scope inconsistency; resets verification clock.      |
| 7 | Brand verification not started                                                     | Friction | Some assessors require completion before CASA finalization.          |
| 8 | CASA not initiated                                                                 | Blocker (past 100 users) | Required to leave Testing mode for restricted scopes.    |
| 9 | Approaching test-user cap                                                          | Blocker  | New friends cannot be added without bumping someone off.             |
| 10 | Support email unmonitored                                                          | Friction | Founder may miss verification correspondence.                        |

---

## 4. Exact actions

### 4.1 Founder-only (Cloud Console)

| # | Action                                                                                                                       | Owner   | Blocking? |
| - | ---------------------------------------------------------------------------------------------------------------------------- | ------- | --------- |
| F1 | Read publishing status. Record `Testing` or `In production` in next SMOKE_REPORT.                                              | Founder | n/a (read-only) |
| F2 | Read External Test Users list. Record count and exact emails (verify Morris / gm3258 / Sheila + founder all present, no typos). | Founder | Yes      |
| F3 | Read User Type (External vs Internal).                                                                                        | Founder | Yes      |
| F4 | Verify app name, logo, home URL, privacy URL, Terms URL, support email, developer contact email all populated and correct.    | Founder | Yes (for CASA) |
| F5 | Verify Authorized domains match the eTLD+1 of the registered redirect URI and the privacy/home URLs.                          | Founder | Yes      |
| F6 | Verify Console-declared scopes are exactly `https://www.googleapis.com/auth/gmail.readonly` and nothing else.                  | Founder | Yes      |
| F7 | Verify Console-registered redirect URI string === value of production `BREVIO_OAUTH_REDIRECT_URI_GOOGLE` env var (case + trailing slash). | Founder | Yes      |
| F8 | Confirm whether CASA was previously initiated, with which assessor, and at which step.                                         | Founder | Yes (gate for §5) |
| F9 | Record current OAuth user gauge from Console.                                                                                  | Founder | n/a       |
| F10 | Add any planned next friend's Google account to External Test Users *before* briefing them (founder rule: briefing before invite mint, per [[v05-2-real-friend-scope]]). | Founder | Yes (per-friend) |

### 4.2 Claude-safe (docs/ops, no runtime code)

| # | Action                                                                                                                                            | Owner  |
| - | ------------------------------------------------------------------------------------------------------------------------------------------------- | ------ |
| C1 | Grep for `gmail.readonly` literals to confirm only one scope is referenced anywhere in the runtime (already done — see §1.1).                       | Claude |
| C2 | Grep for env var name references (`GOOGLE_CLIENT_ID`, `GOOGLE_CLIENT_SECRET`, `BREVIO_OAUTH_REDIRECT_URI_GOOGLE`) to confirm no shadow code paths.   | Claude |
| C3 | Audit `apps/fomo/src/**` for any test fixture or smoke script that accidentally contains a real client ID, secret, or KEK.                          | Claude |
| C4 | Confirm no other Google API surface is wired (Calendar, Drive, Contacts) by grepping the adapters tree (`apps/fomo/src/adapters/`).                  | Claude |
| C5 | Update `docs/runbooks/` and the v0.5.x SMOKE_REPORT_TEMPLATEs to surface a `Google Cloud Console state` block the founder fills in pre-smoke.        | Claude |
| C6 | Keep this doc in sync when env-var names, scope, route paths, or crypto posture change in code.                                                     | Claude |

Out-of-scope for Claude this workflow: any change under `apps/fomo/src/`, `apps/fomo/src/security/oauth/`, route handlers, or Google client construction.

---

## 5. External wait time

- CASA (Cloud Application Security Assessment) is required for OAuth apps requesting Google-classified *restricted scopes*. `gmail.readonly` is on Google's restricted-scope list **TO VERIFY (founder, current Google policy page)** — the classification has been stable historically but Google revises it.
- Community-reported CASA tier-2 timelines for restricted scopes run roughly **2 to 8 weeks** end-to-end (assessor onboarding + evidence collection + Google re-review). **TO VERIFY** — Google does not publish an SLA, and timelines vary by assessor and how clean the submission is.
- The founder rule "fastest safe proof, not maximum possible proof" applies here: do not start CASA speculatively. Initiate it only when there is a concrete plan to cross the test-user cap *and* §4.1 F4–F7 are clean.
- While in Testing mode, the unverified-app interstitial is *Google-controlled*. The right fix is verification, not an in-app warning to the user (see §7).

---

## 6. Decision triggers

| Trigger                                                                                  | Effect                                                                                          |
| ---------------------------------------------------------------------------------------- | ----------------------------------------------------------------------------------------------- |
| Founder confirms a specific new friend's Google email is added to External Test Users     | That friend is *immediately* unblocked from granting consent. No CASA dependency.               |
| Test-user gauge (§4.1 F9) is within 10 of the published cap                               | Begin §4.1 F4–F7 cleanup so a CASA submission is ready when needed.                              |
| Test-user gauge hits the cap *and* there is product demand to cross it                   | Initiate CASA. Until CASA approval lands, *no additional friend can be onboarded*.               |
| CASA approval lands (`In production` publishing)                                          | Unverified-app interstitial disappears for all users; test-user list ceases to gate onboarding. |
| Console-declared scopes drift from `gmail.readonly`                                       | Hard stop — re-align Console to code before next smoke; otherwise verification clock resets.    |
| Authorized domains / redirect URI mismatch detected                                       | Hard stop — fix in Console before next smoke; otherwise callback returns `redirect_uri_mismatch`. |

---

## 7. What NOT to build yet

- **No scope additions or changes.** Do not propose `gmail.modify`, `gmail.send`, `gmail.labels`, `gmail.metadata`, full-mailbox, Calendar, Drive, or People. Every additional scope re-opens verification and adds CASA evidence requirements. Brevio's product layers (HMR, PIL, Feedback substrate) all currently work with read-only Gmail. See [[email-context-provider-abstraction]] for the long-term multi-provider note — still not v0.5.
- **No new Google API integrations.** No Calendar adapter, no Drive adapter, no Contacts adapter — even if scope is already broad enough, the surface area would re-trigger verification and runtime hardening.
- **No self-serve invite flow that bypasses Google verification.** Friend onboarding will hit the same Google interstitial regardless of how slick Brevio's invite UX is. Verification is the only real unblocker.
- **No in-app warning to the user about the unverified-app screen.** The warning is *Google's*; the fix is *at Google*. Reproducing the warning inside Brevio adds nothing the briefing script doesn't already cover ([[v05-2-real-friend-scope]]: briefing-before-invite-mint), and it normalizes a UX we want to eliminate.
- **No automated CASA-status polling.** No web scraping, no Console API integration, no LLM-based "are we approved yet" check. The founder reads Console.
- **No founder command surface scoping in this doc.** Out of scope per workflow boundaries.

---

## 8. Reference links

**Code (cite when reading or reviewing)**

- [apps/fomo/src/security/oauth/providers/index.ts](apps/fomo/src/security/oauth/providers/index.ts) — provider registry, env-var bindings, `buildAuthorizeUrl`.
- [apps/fomo/src/security/oauth/state.ts](apps/fomo/src/security/oauth/state.ts) — state HMAC + PKCE verifier helpers + nonce store interface.
- [apps/fomo/src/security/oauth/exchange.ts](apps/fomo/src/security/oauth/exchange.ts) — code-for-token exchange.
- [apps/fomo/src/security/oauth/token-store.ts](apps/fomo/src/security/oauth/token-store.ts) — encrypted token persistence.
- [apps/fomo/src/security/oauth/refresh-helper.ts](apps/fomo/src/security/oauth/refresh-helper.ts) — refresh-token flow.
- [apps/fomo/src/security/token-crypto.ts](apps/fomo/src/security/token-crypto.ts) — AES-256-GCM primitives, KEK loading.
- [apps/fomo/src/routes/oauth-google.ts](apps/fomo/src/routes/oauth-google.ts) — `POST /oauth/google/start` + `GET /oauth/google/callback`.
- [apps/fomo/src/routes/onboard.ts](apps/fomo/src/routes/onboard.ts) — `/onboard` invite flow reusing same OAuth substrate.
- [apps/fomo/src/adapters/gmail/client.ts](apps/fomo/src/adapters/gmail/client.ts) — `GMAIL_READONLY_SCOPE`, Gmail HTTP client.

**Memory**

- [[brevio-oauth-google-reauth-procedure]] — exact session-mint → curl → browser → callback sequence for founder re-auth.
- [[v05-2-pass]], [[v05-3-pass]], [[v05-4-pass]] — phase-pass entries documenting which Google accounts have successfully granted and what Friend B observed.
- [[v05-2-real-friend-scope]] — "briefing BEFORE invite mint" rule that the verification-warning briefing script lives under.
- [[email-context-provider-abstraction]] — long-term multi-provider direction (NOT v0.5).
- [[no-gate-creep-on-extra-smokes]] — relevant when deciding whether to do another friend smoke before CASA vs after.

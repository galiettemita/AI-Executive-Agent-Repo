# Brevio → FOMO-Killer: Salvage Map

**Branch:** april-may-improvements
**Decision date:** 2026-05-18
**Source:** `/office-hours` session, Approach A (self-first prototype)
**Goal:** Repurpose existing Brevio code as the foundation for a Gmail+Calendar→SMS "don't let me miss what matters" agent. v1 in 2-3 weeks.

---

## The 48-hour decision gate

Codex's challenge to keeping any of this code: *"Six months of pre-product infra often hides abstraction debt, imagined future requirements, and trust/safety machinery that is premature for a wedge this narrow."*

**Rule:** spend day 1-2 trying to build the new Gmail-watch → SMS pipe inside the salvaged code below. At the end of 48 hours, score each module on:

1. **Did it accelerate me?** (helped me skip building X)
2. **Did it fight me?** (forced me to reverse-engineer abstractions, satisfy types I don't need, work around premature safety machinery)

If the answer is mostly (2) for a module — **delete it from this list and replace with a fresh implementation**. Cross it off below and write "killed (date, reason)" next to it.

---

## SALVAGE LIST (keep — these are the head start)

### `services/brevio-brain/src/` — the AI reasoning core (~3.6k non-test lines)

| File | Lines | Why keep |
|---|---|---|
| [services/brevio-brain/src/classify.ts](services/brevio-brain/src/classify.ts) | 216 | Intent classification. The "is this email important?" function lives here. |
| [services/brevio-brain/src/catalog.ts](services/brevio-brain/src/catalog.ts) | 217 | Skill catalog with safety tiers. We'll prune to just `gmail-watch` and `sms-send` for v1. |
| [services/brevio-brain/src/gating.ts](services/brevio-brain/src/gating.ts) | 165 | Deny-by-default policy gate. Genuinely useful — keeps the agent from sending spam texts. |
| [services/brevio-brain/src/verify.ts](services/brevio-brain/src/verify.ts) | 163 | Pre-action verifier. Keep as the "should I send this SMS?" check. |
| [services/brevio-brain/src/policy.ts](services/brevio-brain/src/policy.ts) | 313 | Policy evaluation. Keep but expect to simplify aggressively. |
| [services/brevio-brain/src/types.ts](services/brevio-brain/src/types.ts) | 348 | Shared types. Prune to what v1 actually needs. |
| [services/brevio-brain/src/normalize.ts](services/brevio-brain/src/normalize.ts) | 59 | Input normalization. Small and useful. |
| [services/brevio-brain/src/disambiguate.ts](services/brevio-brain/src/disambiguate.ts) | 219 | **Probably overkill for v1** — disambiguation matters when the agent takes many actions. For "send SMS or don't" it's a single bit. Audit hard. |
| [services/brevio-brain/src/decompose.ts](services/brevio-brain/src/decompose.ts) | 166 | **Probably overkill for v1** — multi-step task decomposition. v1 only does "alert or don't." |
| [services/brevio-brain/src/aggregate.ts](services/brevio-brain/src/aggregate.ts) | 104 | **Probably overkill for v1** — multi-skill aggregation. Audit hard. |
| [services/brevio-brain/src/planner.ts](services/brevio-brain/src/planner.ts) | 611 | **Risk: too abstract for narrow wedge** — built to plan multi-step assistant work. We need ranking, not planning. **Strong candidate for deletion.** |
| [services/brevio-brain/src/workflow-runtime.ts](services/brevio-brain/src/workflow-runtime.ts) | 340 | **Likely Temporal-flavored** — if it depends on Temporal at all, delete. v1 uses a cron worker. |
| [services/brevio-brain/src/index.ts](services/brevio-brain/src/index.ts) | 669 | The brain server entrypoint. Keep but expect heavy rewrite. |
| [services/brevio-brain/src/config.ts](services/brevio-brain/src/config.ts) | 249 | Config loader. Useful. |
| [services/brevio-brain/src/server.test.ts](services/brevio-brain/src/server.test.ts) | (test) | Keep tests for anything you keep. |

**Brain salvage subtotal:** ~3,600 lines of source, ~5,520 with tests.

### `services/brevio-gateway/src/` — auth + OAuth + state (~3.3k non-test lines)

| File | Lines | Why keep |
|---|---|---|
| [services/brevio-gateway/src/oauth-routes.ts](services/brevio-gateway/src/oauth-routes.ts) | 279 | Google OAuth initiate/callback. Gmail+Calendar use these scopes. Real value. |
| [services/brevio-gateway/src/oauth-exchange.ts](services/brevio-gateway/src/oauth-exchange.ts) | 141 | Token exchange. Useful. |
| [services/brevio-gateway/src/oauth-state.ts](services/brevio-gateway/src/oauth-state.ts) | 135 | OAuth state/CSRF. Useful. |
| [services/brevio-gateway/src/oauth-providers/index.ts](services/brevio-gateway/src/oauth-providers/index.ts) | (small) | Google provider config. Useful. |
| [services/brevio-gateway/src/token-store.ts](services/brevio-gateway/src/token-store.ts) | 117 | OAuth token storage. Useful. |
| [services/brevio-gateway/src/crypto.ts](services/brevio-gateway/src/crypto.ts) | 98 | Token encryption at rest. **Don't skip this** — tokens in plaintext is a bad first impression. |
| [services/brevio-gateway/src/auth.ts](services/brevio-gateway/src/auth.ts) | 107 | App-level auth. Useful for the user session. |
| [services/brevio-gateway/src/auth-middleware.ts](services/brevio-gateway/src/auth-middleware.ts) | 60 | Express middleware. Useful. |
| [services/brevio-gateway/src/consent-store.ts](services/brevio-gateway/src/consent-store.ts) | 113 | Consent tracking. Useful when v1 needs explicit "can I read your inbox" toggle. |
| [services/brevio-gateway/src/consent-routes.ts](services/brevio-gateway/src/consent-routes.ts) | 328 | **Audit — built for the chat-UI consent flow that we just killed.** Keep the store, rewrite the routes for a minimal "yes, watch my inbox" toggle. |
| [services/brevio-gateway/src/rate-limit.ts](services/brevio-gateway/src/rate-limit.ts) | 72 | Useful. |
| [services/brevio-gateway/src/safe-logger.ts](services/brevio-gateway/src/safe-logger.ts) | 93 | PII-aware logging. **Definitely keep** — when you ship to friends, you don't want their email bodies in your logs. |
| [services/brevio-gateway/src/security.ts](services/brevio-gateway/src/security.ts) | 36 | Tiny, useful. |
| [services/brevio-gateway/src/audit.ts](services/brevio-gateway/src/audit.ts) | 68 | Append-only audit log. Useful for "what did the agent do today?" debugging. |
| [services/brevio-gateway/src/config.ts](services/brevio-gateway/src/config.ts) | 39 | Useful. |
| [services/brevio-gateway/src/types.ts](services/brevio-gateway/src/types.ts) | 88 | Useful. |
| [services/brevio-gateway/src/format.ts](services/brevio-gateway/src/format.ts) | 31 | Useful. |
| [services/brevio-gateway/src/state.ts](services/brevio-gateway/src/state.ts) | 150 | Useful. |
| [services/brevio-gateway/src/normalize.ts](services/brevio-gateway/src/normalize.ts) | 200 | **Audit — built for the chat-UI brain/process flow.** Some pieces relevant, some not. |
| [services/brevio-gateway/src/pending-message-store.ts](services/brevio-gateway/src/pending-message-store.ts) | 89 | **Probably delete** — built for the "resume-on-auth" chat UX from the April 25 design doc. Not relevant for SMS flow. |
| [services/brevio-gateway/src/workflow-runtime.ts](services/brevio-gateway/src/workflow-runtime.ts) | 91 | **Probably delete** — Temporal/workflow-flavored. v1 doesn't need this. |
| [services/brevio-gateway/src/index.ts](services/brevio-gateway/src/index.ts) | 827 | Server entrypoint. Heavy rewrite expected. |

**Gateway salvage subtotal:** ~3,300 lines of source, ~4,643 with tests.

### `packages/shared/src/` — types and contracts (~800 lines)

| File | Lines | Why keep |
|---|---|---|
| [packages/shared/src/skill-tiers.ts](packages/shared/src/skill-tiers.ts) | 119 | Safety tier definitions. Conceptually useful even if v1 only has 2 skills. |
| [packages/shared/src/capability-inventory.ts](packages/shared/src/capability-inventory.ts) | 214 | **Probably overkill for v1** — designed for runtime capability discovery across many services. v1 has one binary. |
| [packages/shared/src/schemas/skill-result.ts](packages/shared/src/schemas/skill-result.ts) | 104 | Useful. |
| [packages/shared/src/schemas/a2a-runtime.ts](packages/shared/src/schemas/a2a-runtime.ts) | 190 | **Probably delete** — agent-to-agent runtime contracts. v1 has one agent. |
| [packages/shared/src/schemas/message-envelope.ts](packages/shared/src/schemas/message-envelope.ts) | 93 | Useful if we keep multi-process design; delete if we collapse to one process. |
| [packages/shared/src/interfaces/skill-adapter.ts](packages/shared/src/interfaces/skill-adapter.ts) | 60 | Useful — defines the skill plugin shape. |
| [packages/shared/src/errors/index.ts](packages/shared/src/errors/index.ts) | 7 | Tiny, keep. |

**Shared salvage subtotal:** ~800 lines of source, ~2,900 with tests.

### Migrations — the only DB code worth keeping

| File | Why keep |
|---|---|
| [migrations/012_consent_audit_oauth.up.sql](migrations/012_consent_audit_oauth.up.sql) | OAuth token table + consent audit. Use as-is. |
| [migrations/012_consent_audit_oauth.down.sql](migrations/012_consent_audit_oauth.down.sql) | Matching down migration. |

Audit `migrations/001_*` through `migrations/011_*` — keep only the tables you actually use (users, sessions, oauth_tokens, consent). Everything else can be parked.

### Build/tooling (keep)

| File | Why keep |
|---|---|
| [package.json](package.json) | Monorepo root. Keep, prune deps. |
| [pnpm-workspace.yaml](pnpm-workspace.yaml) | Monorepo config. Keep. |
| [tsconfig.base.json](tsconfig.base.json) | TS config. Keep. |
| [turbo.json](turbo.json) | Turborepo. Keep if you still want monorepo; delete if you collapse to one package. |

---

## PARK / DELETE LIST (move to a `legacy/` branch, do not bring forward)

### Top-level Go binaries (~300 MB compiled artifacts)

These are checked-in compiled binaries. **They should not be in git in the first place.** Delete from working tree:

- `agents` (14M)
- `brain` (68M)
- `brevioctl` (14M)
- `browser` (9.5M)
- `canvas` (8.4M)
- `control` (22M)
- `cron` (14M)
- `executor` (23M)
- `hands` (9.8M)
- `marketing` (14M)
- `memory` (14M)
- `router` (14M)
- `temporal-worker` (75M)

Add to `.gitignore`. Use `git filter-repo` later to scrub history if size matters.

### Go source

- [cmd/](cmd/) — 6 service entrypoints. Delete from working tree, archive in `legacy/` branch.
- [internal/](internal/) — 49k lines of Go. Most of it duplicates the TS side (onboarding, connectors, identity, observability). Archive.

### Infrastructure (premature for pre-product)

- [helm/](helm/) — 63 files. Kubernetes deployment. **Delete.** v1 deploys to Fly.io / Railway / Render.
- [terraform/](terraform/) — 62 files. AWS infra. **Delete.** Use managed Postgres + Redis.
- [policies/](policies/) — 14 OPA files. **Delete.** TS `verify.ts` already handles policy at the level v1 needs.
- [helm/](helm/), [terraform/](terraform/), `Dockerfile`, `docker-compose.yml` — **delete or simplify.** v1 needs one Procfile.

### Chat-UI-specific code (from the April 25 design doc which is now superseded)

- The "transparency card" + "inline OAuth pre-warm" + "settings page" frontend contract — there is no chat UI now. SMS replaces it.
- [services/brevio-gateway/src/pending-message-store.ts](services/brevio-gateway/src/pending-message-store.ts) — built for resume-on-auth chat UX. Delete.

### Docs / scratch / artifacts (move to a `notes/` folder, don't delete)

- `Brevio_V9*.docx`, `Brevio_V91*.docx`, `Brevio_V92*.docx`
- `Deterministic*.pdf`
- `Simulator_vs_Production_Gap_Analysis.pdf`
- `CHECKLIST.md` (253KB!), `DECISION.md` (254KB!) — these are giant. Move to a `legacy-notes/` folder.
- `FULL_FILE_INVENTORY*.txt`, `CODEBASE_INVENTORY.md`, `GAP_ANALYSIS.md`, `P0B_FORENSIC_AUDIT_REPORT.md` — same.
- `AI Agent (3).zip`, `AI Agent (5).zip` — delete.

---

## TOTAL SALVAGE BUDGET

| Category | Source lines (non-test) | Risk of premature abstraction |
|---|---|---|
| Brain (15 files) | ~3,600 | Medium-high (planner, workflow-runtime, decompose, aggregate are suspect) |
| Gateway (22 files) | ~3,300 | Low-medium (most pieces map cleanly; consent-routes + pending-message + workflow-runtime are suspect) |
| Shared (7 files) | ~800 | Low (small files, easy to evaluate) |
| **Total** | **~7,700** | |

Plus tests (~5,500 lines) for confidence the salvaged code still works after pruning.

**Note:** earlier I quoted ~13k TS lines. That counted tests. The non-test source is ~7.7k. Cleaner number.

---

## 48-HOUR DECISION CHECKLIST

At the end of day 2, go through this list. Mark each with **KEEP**, **PRUNE** (use a small subset), or **KILL**:

- [ ] `brain/index.ts` (heavy entrypoint) — KEEP / PRUNE / KILL
- [ ] `brain/planner.ts` — KEEP / PRUNE / KILL
- [ ] `brain/workflow-runtime.ts` — KEEP / PRUNE / KILL
- [ ] `brain/disambiguate.ts` — KEEP / PRUNE / KILL
- [ ] `brain/decompose.ts` — KEEP / PRUNE / KILL
- [ ] `brain/aggregate.ts` — KEEP / PRUNE / KILL
- [ ] `gateway/index.ts` (heavy entrypoint) — KEEP / PRUNE / KILL
- [ ] `gateway/consent-routes.ts` — KEEP / PRUNE / KILL
- [ ] `gateway/normalize.ts` — KEEP / PRUNE / KILL
- [ ] `gateway/pending-message-store.ts` — KEEP / PRUNE / KILL
- [ ] `gateway/workflow-runtime.ts` — KEEP / PRUNE / KILL
- [ ] `shared/capability-inventory.ts` — KEEP / PRUNE / KILL
- [ ] `shared/schemas/a2a-runtime.ts` — KEEP / PRUNE / KILL

For anything you mark KILL: replace with a fresh, minimal implementation in a new file. Don't preserve the abstraction.

If by end of day 2 you've marked more than 6 things KILL — Codex was right and you should restart from a fresh repo, importing only the OAuth + crypto + safe-logger files.

---

## WHAT TO ADD (new code, not salvage)

| Component | Approx lines | Notes |
|---|---|---|
| Sendblue iMessage adapter (send + inbound webhook) | ~150 | One file. Sendblue REST API for send; their webhook calls our `/inbound` with reply body + sender. SMS Plus fallback handled transparently by Sendblue. |
| Gmail watch worker (cron-driven, ingests last N minutes) | ~250 | Polls Gmail API, hands new messages to brain |
| Memory store (sender importance, suppressions, user prefs) | ~200 | Plain Postgres table + a few read/write helpers |
| Reply parser (parse `open`, `later`, `tomorrow`, `ignore`, `ignore from <sender>`, `why`, `STOP`) | ~150 | Small state machine, regex first, LLM fallback for fuzzy sender matching |
| Ranking model wrapper ("would user be sad to miss this?") | ~200 | LLM call + prompt |
| Internal review queue (founder inspects alerts before send) | ~150 | Web page + table — batched at 9am/6pm windows |
| Onboarding (3 SMS to set up name/Gmail/preferences) | ~200 | |

**New code budget: ~1,300 lines.**

**Deferred (NOT in v1 new code):** Calendar watch worker (~150 lines) — Gmail-only for v1. Add in Phase 2 if Gmail wedge validates.

---

## DEPLOYMENT TARGET (v1)

- **Runtime:** Fly.io or Railway, single Node process + a cron worker
- **DB:** Postgres (managed, Fly Postgres or Neon)
- **Cache/queue:** Redis (Upstash) — optional for v1
- **Outbound iMessage / SMS:** Sendblue (iMessage native to Apple users, SMS Plus fallback for Android)
- **Inbound webhook:** Sendblue inbound → POST to `/inbound` route

**Not used in v1:** Kubernetes, Helm, Terraform, AWS, Temporal, OPA.

---

## SUCCESS CRITERION (week 4 review)

Before investing another month past v1, the prototype must prove:

1. **You** use it daily for 14 days and stop checking your inbox out of fear.
2. **At least 3 friends** use it for 7 days and don't churn.
3. **At least 1 friend** says "keep this running, I'd miss it if it stopped."

If yes → continue, expand to Calendar + more users. If no → the wedge is wrong or the ranking quality is too low. Either way: real data, not a vibe.

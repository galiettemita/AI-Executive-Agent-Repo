# Brevio Skills / Mistake Rulebook

This file is Brevio institutional knowledge for Hermes/Codex/Claude operating mistakes and new rules. Each time Galiette gives corrective feedback, append an entry with:

- Date
- Exact error
- Root cause
- Abstraction of the error
- New rule
- Verification / operating hook

The purpose is compounding operational memory: repeated mistakes become rules, checks, prompts, tests, or harness updates instead of relying on any single session's memory.

## Required Brevio session-start loop

Every Brevio session and every autonomous Brevio PM cycle begins with this loop before implementation, review, merge, or worker prompting:

1. **Brevio Identity Lock** — confirm this is Brevio work; do not infer from the active Hermes profile/cwd.
2. **Source-of-Truth Intake** — read the founding PDFs, `docs/brevio-core-agent-dimensions.md`, `CLAUDE.md`, `FOMO_DESIGN.md`, `FOMO_PLAN.md`, `apps/fomo/KERNEL.md`, and active phase docs.
3. **Harness Anchor Check** — verify `BREVIO-HARNESS-V1-NO-CIRCLING-FAST-SHIPPING` with `.hermes/verify-brevio-harness.sh`.
4. **Operating Contract Load** — read `.hermes/BREVIO_OPERATING_CONTRACT.md` and `.hermes/project-management-cycle.prompt.md`.
5. **Repo Reality Check** — inspect branch, dirty files, latest commits, relation to `origin/main`, diffs, and PR/CI state if relevant.
6. **Worker State Check** — read the active coding-worker config and decide whether Hermes is reviewing, redirecting, merging, or doing a narrow direct fix.
7. **Scope/Gate Filter** — classify the next action against founding docs, current phase, M1 boundaries, and founder-gated items.
8. **Single Next Action Selection** — choose exactly one concrete move: validate, inspect diff, fix caused failure, review PR, merge verified work, prompt worker, or identify a real founder gate.
9. **Implementation Lane** — code only in `/Users/galiettemita/Projects/Brevio/backend` on a narrow branch; Telegram is the control surface, not the source artifact.
10. **Validation Gate** — run exact relevant tests/lint/build and separate caused failures from pre-existing failures.
11. **Merge Decision Gate** — end every work item with merge now / merge after named checks / do not merge because named blocker / close stale.
12. **Feedback Capture / Rulebook Update** — append Galiette corrections here as dated institutional rules.

## Entry format

```md
## YYYY-MM-DD — Short mistake name

- Exact error:
- Root cause:
- Abstraction:
- New rule:
- Verification / operating hook:
```

---

## 2026-06-26 — Brevio context-boundary failure

- Exact error: Galiette said “continue” while intending Brevio, and Hermes continued the Local Business Tech Upgrade workflow, searched for local businesses, and used the wrong project context.
- Root cause: Hermes inferred intent from the active Telegram profile/current working directory (`localbusinesswork` / `local-business-tech-upgrade`) instead of first anchoring on Brevio signals, prior Brevio context, and the requested Brevio operating workflow.
- Abstraction: Cross-project context bleed. A session running under one Hermes profile can receive a task for a different project; current cwd/profile is not sufficient evidence of user intent.
- New rule: At the start of any Brevio session or autonomous Brevio cycle, perform the Brevio grounding loop before any product work: identify Brevio intent, switch mental/workdir context to `/Users/galiettemita/Projects/Brevio/backend`, read the founding docs and core dimensions doc, verify the harness anchor, read the operating contract, inspect repo state, inspect active worker, then choose exactly one allowed next action. If Brevio intent is ambiguous, ask a one-line clarification instead of continuing another project.
- Verification / operating hook: Brevio PM cycles must read this file, include the harness anchor `BREVIO-HARNESS-V1-NO-CIRCLING-FAST-SHIPPING`, and report the current repo branch/state before claiming readiness or continuing implementation.

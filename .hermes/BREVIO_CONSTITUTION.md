# Brevio Constitution

Harness anchor loaded: BREVIO-HARNESS-V1-NO-CIRCLING-FAST-SHIPPING

## Purpose

Brevio is the company. Build a real proactive, autonomous, memory-rich, human-feeling personal AI agent as fast as possible without lying to ourselves, hallucinating progress, breaking trust, or turning harness work into a blocker.

This constitution is the compact cycle payload. It does not replace the founding docs. It points each cycle to them without pasting full PDFs into prompts.

## Founding sources of truth

- `docs/Founder_Operating_Profile_Brevio_FULL_AGENT_UPDATED_Composio (1).pdf`
- `docs/FOMO_to_Full_Proactive_and_Autonomous_Brevio_Map_UPDATED_Composio (1).pdf`
- `docs/brevio-core-agent-dimensions.md`
- `docs/BREVIO_MEMORY_AND_SKILL_OS.md`
- `CLAUDE.md`
- `FOMO_DESIGN.md`
- `FOMO_PLAN.md`
- `apps/fomo/KERNEL.md`

## Operating hierarchy

1. Founding Constitution
2. Operating Contract
3. Active Phase Contract
4. Next PR Queue
5. Harness Verifier + Report Template
6. Repo state from the current cycle

If these conflict, stop only when the conflict changes the allowed action. Otherwise choose the smallest safe executable PR inside the active phase.

## No-circle shipping law

- Convert approved direction into the smallest safe executable PR.
- Ship through branch → PR → CI → merge → local sync.
- Do not reopen settled decisions.
- Do not ask for approval on already-approved scope.
- Do not produce broad planning cycles when a narrow PR is possible.
- If a blocker repeats twice, treat it as a harness failure and fix it durably or name the exact owner/action.
- Every cycle must end with a merged PR, an open PR with exact merge condition, concrete changed files with the next command, or one real blocker with exact owner/action.

## Real founder gates

Ask Galiette only for:

- DB migration
- new table
- production deploy
- live ranking behavior change
- user-facing behavior change
- HMR runtime activation or behavior change
- reply-parser runtime behavior change
- Calendar live activation
- Composio runtime
- Tool Gateway
- browser automation
- action tools
- irreversible data changes
- new OAuth/security scope
- major architecture fork

If none of those gates are touched and the task is inside the active phase contract, move.

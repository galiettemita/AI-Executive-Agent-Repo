# Brevio Active Phase Contract

Harness anchor loaded: BREVIO-HARNESS-V1-NO-CIRCLING-FAST-SHIPPING

## Current phase

Memory V1 Visible Behavior

## Phase purpose

Make Brevio’s memory feel real to the user through the smallest safe visible behaviors.

## Memory V1 exit condition

Memory V1 Visible Behavior advances these exit conditions:

1. remember explicit user preferences;
2. retrieve relevant memories safely;
3. explain why a memory was used;
4. forget or correct a memory;
5. prove source/audit metadata;
6. prevent cross-user leakage;
7. expose at least one visible memory behavior to the user.

## Founder decision

M1-B / Memory V1 closeout foundation work is complete enough. Do not reopen M1 A/B/C. Do not continue hidden M1-B hardening unless a specific blocker appears. Do not start another broad memory architecture plan. Do not create more harness/meta work unless a repeated real blocker proves a gap.

The approved next phase is Memory V1 Visible Behavior. Convert it into small executable PRs.

## Allowed without fresh founder approval

- explicit memory command behavior such as “remember this”;
- retrieval of explicit preferences into safe assistant context;
- “why did you use this memory?” explanation;
- “forget this” / “correct this” command path, if narrow and safe;
- user-visible memory evidence or debug surface, if small;
- tests proving source/audit metadata;
- tests proving cross-user isolation;
- tests proving deleted/tombstoned memory is not used;
- tests proving private values do not leak into audit/log metadata;
- dormant helpers only when directly tied to a visible Memory V1 PR;
- PR creation, CI wait, merge after green checks, and local sync for in-scope work.

## Forbidden without fresh founder approval

- DB migration;
- new table/schema;
- production deploy;
- live ranking behavior change unrelated to the specific Memory V1 task;
- broad HMR rewrite;
- Calendar live activation;
- Composio runtime;
- Tool Gateway;
- browser automation;
- action tools;
- irreversible data changes;
- OAuth/security scope changes;
- broad strategic phase fork.

## Every task requires exit condition

Before any task or PR begins, define:

1. task name;
2. purpose;
3. allowed scope;
4. forbidden scope;
5. expected changed files/areas;
6. tests/validation required;
7. exit condition;
8. stop condition;
9. next task after completion.

If the exit condition cannot be defined, do not start. If the exit condition is met, stop. Do not keep expanding the task.

## Immediate next PR requirements

The queue must contain exactly one `NEXT` item. The current `NEXT` item must be a concrete Memory V1 Visible Behavior implementation slice, not vague “continue memory” work and not hidden M1-B foundation work.

## Loop prevention

No cycle may report vague work such as “continue M1,” “continue Memory V1,” “improve memory,” “harden the system,” “recommend next phase,” or “review the docs.” The cycle must name the current `NEXT` queue item from `.hermes/NEXT_PR_QUEUE.md` and either ship it, open a PR for it, produce concrete changed files for it, or name the exact blocker and owner/action.

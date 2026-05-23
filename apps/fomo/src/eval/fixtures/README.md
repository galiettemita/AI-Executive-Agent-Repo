# Eval Fixtures

This directory is intentionally empty in Phase 2D.

The eval harness ([`../harness.ts`](../harness.ts)) ships in Phase 2D as
substrate. Real fixtures land in **Phase 3**, alongside the ranker prompt
and the first real model backends, when the bake-off described in
[FOMO_PLAN.md §14](../../../../../../FOMO_PLAN.md) actually runs.

When fixtures arrive they must follow:

1. **No raw user email bodies** in repo. Use anonymized, synthetic, or
   local-only fixtures (FOMO_PLAN §14.3).
2. **One file per fixture set**, named by the prompt+model combination
   it covers, e.g. `ranker-v1.fixtures.json`.
3. **Versioned by prompt_version** — when the prompt changes, copy the
   fixture set, bump the version, and let both sets coexist so regression
   evals are possible.
4. **Fixture format** — array of objects with `prompt` (string) and
   `expected_label` (string) per [`EvalFixture`](../harness.ts).

If you find yourself reaching for real user emails to build fixtures,
stop. Use [FOMO_DESIGN.md §22](../../../../../../FOMO_DESIGN.md) as the
checklist: anonymize sender, anonymize subject, anonymize body, never
commit.

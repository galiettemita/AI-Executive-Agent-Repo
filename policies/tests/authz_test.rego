package brevio.authz_test

import data.brevio.authz

base_input := {
  "user": {
    "tier": "pro",
    "enabled_skills": ["todoist"],
    "monthly_llm_used_cents": 100,
    "monthly_llm_budget_cents": 1000
  },
  "skill": {
    "id": "todoist",
    "enabled": true,
    "min_tier": "free"
  },
  "estimated_cost": 10,
  "request_count_in_window": 1,
  "token": {"user_id": "u1"},
  "requesting_user_id": "u1",
  "requesting_user": {"role": "user"},
  "circuit_breaker": {"state": "CLOSED"},
  "oauth_token": {"expires_at": "2099-01-01T00:00:00Z"},
  "now": "2026-03-03T00:00:00Z",
  "refresh_failed": false
}

test_allow_skill_execution {
  authz.allow_skill_execution with input as base_input
}

test_deny_disabled_skill {
  authz.deny_disabled_skill == "DENY" with input as object.union(base_input, {"skill": object.union(base_input.skill, {"enabled": false})})
}

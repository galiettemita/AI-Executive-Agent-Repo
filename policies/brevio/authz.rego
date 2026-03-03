package brevio.authz

default allow_skill_execution := false
default allow_budget_spend := false
default allow_rate := false
default allow_oauth_access := false

tier_rank := {"free": 0, "pro": 1, "enterprise": 2, "admin": 3}

tier_rate_limit := {"free": 30, "pro": 120, "enterprise": 1000000, "admin": 1000000, "service": 1000000}

allow_skill_execution {
  tier_rank[input.user.tier] >= tier_rank[input.skill.min_tier]
  input.skill.id in input.user.enabled_skills
  input.skill.enabled == true
}

allow_budget_spend {
  input.user.monthly_llm_used_cents + input.estimated_cost <= input.user.monthly_llm_budget_cents
}

allow_rate {
  input.request_count_in_window < tier_rate_limit[input.user.tier]
}

allow_oauth_access {
  input.token.user_id == input.requesting_user_id
}

allow_oauth_access {
  input.requesting_user.role == "admin"
}

deny_disabled_skill := "DENY" {
  input.skill.enabled == false
}

deny_circuit_open := "CIRCUIT_OPEN" {
  input.circuit_breaker.state == "OPEN"
}

deny_expired_token := "AUTH_EXPIRED" {
  input.oauth_token.expires_at < input.now
  input.refresh_failed == true
}

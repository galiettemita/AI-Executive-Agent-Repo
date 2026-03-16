package brevio.authz

default allow_skill_execution := false
default allow_budget_spend := false
default allow_rate := false
default allow_oauth_access := false
default allow_resource_access := false

tier_rank := {"free": 0, "pro": 1, "enterprise": 2, "admin": 3}

tier_rate_limit := {"free": 30, "pro": 120, "enterprise": 1000000, "admin": 1000000, "service": 1000000}

user_tier := lower(object.get(input.user, "tier", "free"))
user_role := lower(object.get(input.user, "role", user_tier))

is_admin {
  user_role == "admin"
}

allow_skill_execution {
  object.get(tier_rank, user_tier, -1) >= object.get(tier_rank, lower(object.get(input.skill, "min_tier", "enterprise")), 99)
  some i
  input.user.enabled_skills[i] == input.skill.id
  input.skill.enabled == true
}

allow_budget_spend {
  input.user.monthly_llm_used_cents + input.estimated_cost <= input.user.monthly_llm_budget_cents
}

allow_rate {
  input.request_count_in_window < object.get(tier_rate_limit, user_tier, 0)
}

allow_oauth_access {
  input.token.user_id == input.requesting_user_id
}

allow_oauth_access {
  is_admin
}

# Access control matrix (resource x role).
allow_resource_access {
  is_admin
}

allow_resource_access {
  input.resource == "core_skills"
  input.action == "read"
  user_tier == "free"
}

allow_resource_access {
  input.resource == "core_skills"
  input.action == "exec"
  user_tier == "free"
}

allow_resource_access {
  input.resource == "core_skills"
  input.action == "read"
  user_tier == "pro"
}

allow_resource_access {
  input.resource == "core_skills"
  input.action == "exec"
  user_tier == "pro"
}

allow_resource_access {
  input.resource == "core_skills"
  input.action == "read"
  user_tier == "enterprise"
}

allow_resource_access {
  input.resource == "core_skills"
  input.action == "exec"
  user_tier == "enterprise"
}

allow_resource_access {
  input.resource == "premium_skills"
  input.action == "read"
  user_tier == "pro"
}

allow_resource_access {
  input.resource == "premium_skills"
  input.action == "exec"
  user_tier == "pro"
}

allow_resource_access {
  input.resource == "premium_skills"
  input.action == "read"
  user_tier == "enterprise"
}

allow_resource_access {
  input.resource == "premium_skills"
  input.action == "exec"
  user_tier == "enterprise"
}

allow_resource_access {
  input.resource == "custom_skill_deploy"
  input.action == "full"
  user_tier == "enterprise"
}

allow_resource_access {
  input.resource == "user_profile_own"
  input.action == "read"
  user_tier == "free"
}

allow_resource_access {
  input.resource == "user_profile_own"
  input.action == "write"
  user_tier == "free"
}

allow_resource_access {
  input.resource == "user_profile_own"
  input.action == "read"
  user_tier == "pro"
}

allow_resource_access {
  input.resource == "user_profile_own"
  input.action == "write"
  user_tier == "pro"
}

allow_resource_access {
  input.resource == "user_profile_own"
  input.action == "read"
  user_tier == "enterprise"
}

allow_resource_access {
  input.resource == "user_profile_own"
  input.action == "write"
  user_tier == "enterprise"
}

allow_resource_access {
  input.resource == "user_profile_others"
  input.action == "read"
  user_tier == "enterprise"
  input.scope == "team"
}

allow_resource_access {
  input.resource == "oauth_tokens_own"
  input.action == "read"
  user_tier == "free"
}

allow_resource_access {
  input.resource == "oauth_tokens_own"
  input.action == "write"
  user_tier == "free"
}

allow_resource_access {
  input.resource == "oauth_tokens_own"
  input.action == "read"
  user_tier == "pro"
}

allow_resource_access {
  input.resource == "oauth_tokens_own"
  input.action == "write"
  user_tier == "pro"
}

allow_resource_access {
  input.resource == "oauth_tokens_own"
  input.action == "read"
  user_tier == "enterprise"
}

allow_resource_access {
  input.resource == "oauth_tokens_own"
  input.action == "write"
  user_tier == "enterprise"
}

allow_resource_access {
  input.resource == "skill_execution_logs"
  input.action == "read"
  user_tier == "free"
  input.scope == "own"
  object.get(input, "lookback_days", 9999) <= 7
}

allow_resource_access {
  input.resource == "skill_execution_logs"
  input.action == "read"
  user_tier == "pro"
  input.scope == "own"
  object.get(input, "lookback_days", 9999) <= 90
}

allow_resource_access {
  input.resource == "skill_execution_logs"
  input.action == "read"
  user_tier == "enterprise"
  input.scope == "team"
  object.get(input, "lookback_days", 9999) <= 90
}

allow_resource_access {
  input.resource == "billing_data"
  input.action == "read"
  user_tier == "free"
  input.scope == "own"
}

allow_resource_access {
  input.resource == "billing_data"
  input.action == "read"
  user_tier == "pro"
  input.scope == "own"
}

allow_resource_access {
  input.resource == "billing_data"
  input.action == "read"
  user_tier == "enterprise"
  input.scope == "team"
}

allow_resource_access {
  input.resource == "workflow_management"
  input.action == "read"
  user_tier == "enterprise"
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

deny_gateway_bypass := "GATEWAY_BYPASS" {
  input.request_target == "brain_direct"
  input.gateway_normalized != true
}

deny_external_api_outside_activity := "ACTIVITY_REQUIRED" {
  input.external_api_call == true
  input.temporal_activity != true
}

deny_pii_log_retention := "PII_RETENTION_VIOLATION" {
  input.log_contains_pii == true
  object.get(input, "log_age_days", 0) > 30
}

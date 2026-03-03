package brevio.authz_test

import data.brevio.authz

base_input := {
  "user": {
    "id": "u1",
    "tier": "pro",
    "role": "user",
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
  deny := authz.deny_disabled_skill with input as object.union(base_input, {
    "skill": object.union(base_input.skill, {"enabled": false})
  })
  deny == "DENY"
}

test_allow_budget_spend {
  authz.allow_budget_spend with input as base_input
}

test_deny_budget_spend {
  not authz.allow_budget_spend with input as object.union(base_input, {
    "estimated_cost": 10000
  })
}

test_allow_rate {
  authz.allow_rate with input as base_input
}

test_deny_rate_for_pro {
  not authz.allow_rate with input as object.union(base_input, {
    "request_count_in_window": 120
  })
}

test_allow_oauth_access_owner {
  authz.allow_oauth_access with input as base_input
}

test_allow_oauth_access_admin {
  authz.allow_oauth_access with input as object.union(base_input, {
    "user": object.union(base_input.user, {"tier": "admin", "role": "admin"})
  })
}

test_deny_oauth_access_other_user {
  not authz.allow_oauth_access with input as object.union(base_input, {
    "token": {"user_id": "u2"},
    "requesting_user_id": "u1",
    "user": object.union(base_input.user, {"tier": "pro", "role": "user"})
  })
}

test_deny_circuit_open {
  deny := authz.deny_circuit_open with input as object.union(base_input, {
    "circuit_breaker": {"state": "OPEN"}
  })
  deny == "CIRCUIT_OPEN"
}

test_deny_expired_token {
  deny := authz.deny_expired_token with input as object.union(base_input, {
    "oauth_token": {"expires_at": "2025-01-01T00:00:00Z"},
    "refresh_failed": true
  })
  deny == "AUTH_EXPIRED"
}

test_deny_gateway_bypass {
  deny := authz.deny_gateway_bypass with input as object.union(base_input, {
    "request_target": "brain_direct",
    "gateway_normalized": false
  })
  deny == "GATEWAY_BYPASS"
}

test_deny_external_api_outside_activity {
  deny := authz.deny_external_api_outside_activity with input as object.union(base_input, {
    "external_api_call": true,
    "temporal_activity": false
  })
  deny == "ACTIVITY_REQUIRED"
}

test_deny_pii_retention_violation {
  deny := authz.deny_pii_log_retention with input as object.union(base_input, {
    "log_contains_pii": true,
    "log_age_days": 31
  })
  deny == "PII_RETENTION_VIOLATION"
}

# Access matrix positive checks.
test_allow_core_skills_free {
  authz.allow_resource_access with input as {
    "user": {"tier": "free", "role": "user"},
    "resource": "core_skills",
    "action": "exec"
  }
}

test_allow_premium_skills_pro {
  authz.allow_resource_access with input as {
    "user": {"tier": "pro", "role": "user"},
    "resource": "premium_skills",
    "action": "exec"
  }
}

test_allow_custom_deploy_enterprise {
  authz.allow_resource_access with input as {
    "user": {"tier": "enterprise", "role": "user"},
    "resource": "custom_skill_deploy",
    "action": "full"
  }
}

test_allow_user_profile_others_enterprise_team {
  authz.allow_resource_access with input as {
    "user": {"tier": "enterprise", "role": "user"},
    "resource": "user_profile_others",
    "action": "read",
    "scope": "team"
  }
}

test_allow_workflow_management_enterprise_read {
  authz.allow_resource_access with input as {
    "user": {"tier": "enterprise", "role": "user"},
    "resource": "workflow_management",
    "action": "read"
  }
}

test_allow_admin_full_access {
  authz.allow_resource_access with input as {
    "user": {"tier": "admin", "role": "admin"},
    "resource": "circuit_breaker_admin",
    "action": "full"
  }
}

# Access matrix explicit denied cells.
test_deny_premium_skills_free {
  not authz.allow_resource_access with input as {
    "user": {"tier": "free", "role": "user"},
    "resource": "premium_skills",
    "action": "exec"
  }
}

test_deny_custom_skill_deploy_free {
  not authz.allow_resource_access with input as {
    "user": {"tier": "free", "role": "user"},
    "resource": "custom_skill_deploy",
    "action": "full"
  }
}

test_deny_custom_skill_deploy_pro {
  not authz.allow_resource_access with input as {
    "user": {"tier": "pro", "role": "user"},
    "resource": "custom_skill_deploy",
    "action": "full"
  }
}

test_deny_user_profile_others_free {
  not authz.allow_resource_access with input as {
    "user": {"tier": "free", "role": "user"},
    "resource": "user_profile_others",
    "action": "read",
    "scope": "team"
  }
}

test_deny_user_profile_others_pro {
  not authz.allow_resource_access with input as {
    "user": {"tier": "pro", "role": "user"},
    "resource": "user_profile_others",
    "action": "read",
    "scope": "team"
  }
}

test_deny_circuit_breaker_admin_free {
  not authz.allow_resource_access with input as {
    "user": {"tier": "free", "role": "user"},
    "resource": "circuit_breaker_admin",
    "action": "full"
  }
}

test_deny_circuit_breaker_admin_pro {
  not authz.allow_resource_access with input as {
    "user": {"tier": "pro", "role": "user"},
    "resource": "circuit_breaker_admin",
    "action": "full"
  }
}

test_deny_circuit_breaker_admin_enterprise {
  not authz.allow_resource_access with input as {
    "user": {"tier": "enterprise", "role": "user"},
    "resource": "circuit_breaker_admin",
    "action": "full"
  }
}

test_deny_workflow_management_free {
  not authz.allow_resource_access with input as {
    "user": {"tier": "free", "role": "user"},
    "resource": "workflow_management",
    "action": "read"
  }
}

test_deny_workflow_management_pro {
  not authz.allow_resource_access with input as {
    "user": {"tier": "pro", "role": "user"},
    "resource": "workflow_management",
    "action": "read"
  }
}

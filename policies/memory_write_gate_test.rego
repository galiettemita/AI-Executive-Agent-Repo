package brevio.memory_write_gate

# Valid enterprise baseline — all rules should pass
enterprise_baseline := {
    "workspace_id": "ws-enterprise-abc",
    "user_id": "user-1",
    "memory_type": "episodic",
    "data_class": "internal",
    "sensitivity_label": "standard",
    "retention_policy_id": "",
    "allowed_processors": ["brevio-default"],
    "workspace_tier": "enterprise",
    "memory_write_blocked": false,
    "content_byte_size": 1000,
    "ttl_override_days": 0
}

# BASELINE
test_allow_valid_enterprise {
    allow with input as enterprise_baseline
}

test_deny_empty_for_valid_enterprise {
    count(deny) == 0 with input as enterprise_baseline
}

# RULE 1: Legacy hard-block
test_deny_memory_write_blocked_true {
    d := deny with input as object.union(enterprise_baseline, {"memory_write_blocked": true})
    d[_] == "MEMORY_WRITE_BLOCKED: explicit block flag is set on this write request"
}

test_allow_memory_write_blocked_false {
    allow with input as object.union(enterprise_baseline, {"memory_write_blocked": false})
}

# RULE 2: Restricted data class
test_deny_restricted_on_pro_tier {
    d := deny with input as object.union(enterprise_baseline, {
        "data_class": "restricted",
        "workspace_tier": "pro"
    })
    some msg in d
    startswith(msg, "RESTRICTED_REQUIRES_ENTERPRISE")
}

test_deny_restricted_on_free_tier {
    d := deny with input as object.union(enterprise_baseline, {
        "data_class": "restricted",
        "workspace_tier": "free",
        "allowed_processors": ["brevio-default"],
        "memory_type": "episodic"
    })
    some msg in d
    startswith(msg, "RESTRICTED_REQUIRES_ENTERPRISE")
}

test_allow_restricted_on_enterprise {
    d := deny with input as object.union(enterprise_baseline, {"data_class": "restricted"})
    not any_starts_with(d, "RESTRICTED_REQUIRES_ENTERPRISE")
}

# RULE 3: Regulated + retention policy
test_deny_regulated_no_retention_policy {
    d := deny with input as object.union(enterprise_baseline, {
        "sensitivity_label": "regulated",
        "retention_policy_id": ""
    })
    some msg in d
    startswith(msg, "REGULATED_REQUIRES_RETENTION_POLICY")
}

test_allow_regulated_with_retention_policy {
    d := deny with input as object.union(enterprise_baseline, {
        "sensitivity_label": "regulated",
        "retention_policy_id": "ret-policy-abc-123"
    })
    not any_starts_with(d, "REGULATED_REQUIRES_RETENTION_POLICY")
}

# RULE 4: Allowed processors
test_deny_empty_processors {
    d := deny with input as object.union(enterprise_baseline, {"allowed_processors": []})
    some msg in d
    startswith(msg, "ALLOWED_PROCESSORS_REQUIRED")
}

test_allow_nonempty_processors {
    d := deny with input as object.union(enterprise_baseline, {
        "allowed_processors": ["brevio-default", "claude-haiku"]
    })
    not any_starts_with(d, "ALLOWED_PROCESSORS_REQUIRED")
}

# RULE 5: Free tier processor restriction
test_deny_free_tier_unauthorized_processor {
    d := deny with input as object.union(enterprise_baseline, {
        "workspace_tier": "free",
        "allowed_processors": ["gpt-4"],
        "memory_type": "episodic"
    })
    some msg in d
    startswith(msg, "UNAUTHORIZED_PROCESSOR")
}

test_allow_free_tier_default_processor {
    d := deny with input as object.union(enterprise_baseline, {
        "workspace_tier": "free",
        "allowed_processors": ["brevio-default"],
        "memory_type": "episodic"
    })
    not any_starts_with(d, "UNAUTHORIZED_PROCESSOR")
}

# RULE 6: Premium memory types
test_deny_free_tier_rule_type {
    d := deny with input as object.union(enterprise_baseline, {
        "workspace_tier": "free",
        "memory_type": "rule",
        "allowed_processors": ["brevio-default"]
    })
    some msg in d
    startswith(msg, "MEMORY_TYPE_REQUIRES_PAID_TIER")
}

test_deny_free_tier_preference_type {
    d := deny with input as object.union(enterprise_baseline, {
        "workspace_tier": "free",
        "memory_type": "preference",
        "allowed_processors": ["brevio-default"]
    })
    some msg in d
    startswith(msg, "MEMORY_TYPE_REQUIRES_PAID_TIER")
}

test_allow_pro_tier_rule_type {
    d := deny with input as object.union(enterprise_baseline, {
        "workspace_tier": "pro",
        "memory_type": "rule"
    })
    not any_starts_with(d, "MEMORY_TYPE_REQUIRES_PAID_TIER")
}

test_allow_free_tier_episodic {
    d := deny with input as object.union(enterprise_baseline, {
        "workspace_tier": "free",
        "memory_type": "episodic",
        "allowed_processors": ["brevio-default"]
    })
    not any_starts_with(d, "MEMORY_TYPE_REQUIRES_PAID_TIER")
}

# RULE 7: Content size limits
test_deny_content_exceeds_free_limit {
    d := deny with input as object.union(enterprise_baseline, {
        "workspace_tier": "free",
        "content_byte_size": 6000,
        "allowed_processors": ["brevio-default"],
        "memory_type": "episodic"
    })
    some msg in d
    startswith(msg, "CONTENT_SIZE_EXCEEDED")
}

test_deny_content_exceeds_pro_limit {
    d := deny with input as object.union(enterprise_baseline, {
        "workspace_tier": "pro",
        "content_byte_size": 60000
    })
    some msg in d
    startswith(msg, "CONTENT_SIZE_EXCEEDED")
}

test_allow_content_within_enterprise_limit {
    d := deny with input as object.union(enterprise_baseline, {
        "content_byte_size": 999999
    })
    not any_starts_with(d, "CONTENT_SIZE_EXCEEDED")
}

# RULE 8: TTL floor
test_deny_ttl_below_floor_for_rule {
    d := deny with input as object.union(enterprise_baseline, {
        "memory_type": "rule",
        "ttl_override_days": 30
    })
    some msg in d
    startswith(msg, "TTL_BELOW_FLOOR")
}

test_allow_ttl_zero_is_default {
    d := deny with input as object.union(enterprise_baseline, {
        "memory_type": "rule",
        "ttl_override_days": 0
    })
    not any_starts_with(d, "TTL_BELOW_FLOOR")
}

test_allow_ttl_above_floor {
    d := deny with input as object.union(enterprise_baseline, {
        "memory_type": "preference",
        "ttl_override_days": 365
    })
    not any_starts_with(d, "TTL_BELOW_FLOOR")
}

# RULE 9: Workspace isolation
test_deny_sensitive_in_default_workspace {
    d := deny with input as object.union(enterprise_baseline, {
        "workspace_id": "default",
        "sensitivity_label": "sensitive"
    })
    some msg in d
    startswith(msg, "SENSITIVE_DATA_REQUIRES_ISOLATED_WORKSPACE")
}

test_deny_regulated_in_shared_workspace {
    d := deny with input as object.union(enterprise_baseline, {
        "workspace_id": "shared",
        "sensitivity_label": "regulated",
        "retention_policy_id": "ret-abc"
    })
    some msg in d
    startswith(msg, "SENSITIVE_DATA_REQUIRES_ISOLATED_WORKSPACE")
}

test_allow_sensitive_in_isolated_workspace {
    d := deny with input as object.union(enterprise_baseline, {
        "workspace_id": "ws-isolated-abc123",
        "sensitivity_label": "sensitive"
    })
    not any_starts_with(d, "SENSITIVE_DATA_REQUIRES_ISOLATED_WORKSPACE")
}

# MULTI-VIOLATION
test_multiple_violations_accumulated {
    d := deny with input as object.union(enterprise_baseline, {
        "workspace_tier": "pro",
        "data_class": "restricted",
        "allowed_processors": []
    })
    count(d) >= 2
}

# HELPER
any_starts_with(s, prefix) {
    some msg in s
    startswith(msg, prefix)
}

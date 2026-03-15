package brevio.memory_write_gate

# BREVIO MEMORY WRITE GATE
# Authoritative policy for all memory write operations.
# Input: see memory_write_gate_input.schema.json
# Output: allow (boolean), deny (set of "CODE: reason" strings)
#
# REGISTERED DENY CODES:
# MEMORY_WRITE_BLOCKED
# RESTRICTED_REQUIRES_ENTERPRISE
# REGULATED_REQUIRES_RETENTION_POLICY
# ALLOWED_PROCESSORS_REQUIRED
# UNAUTHORIZED_PROCESSOR
# MEMORY_TYPE_REQUIRES_PAID_TIER
# CONTENT_SIZE_EXCEEDED
# TTL_BELOW_FLOOR
# SENSITIVE_DATA_REQUIRES_ISOLATED_WORKSPACE

default allow := false

allow {
    count(deny) == 0
}

# RULE 1: Legacy hard-block flag
deny[msg] {
    input.memory_write_blocked == true
    msg := "MEMORY_WRITE_BLOCKED: explicit block flag is set on this write request"
}

# RULE 2: Restricted data class requires enterprise tier
deny[msg] {
    input.data_class == "restricted"
    input.workspace_tier != "enterprise"
    msg := sprintf(
        "RESTRICTED_REQUIRES_ENTERPRISE: data_class 'restricted' requires enterprise tier (current: '%v')",
        [input.workspace_tier]
    )
}

# RULE 3: Regulated sensitivity requires retention policy
deny[msg] {
    input.sensitivity_label == "regulated"
    input.retention_policy_id == ""
    msg := "REGULATED_REQUIRES_RETENTION_POLICY: retention_policy_id is empty; regulated data requires an active retention policy"
}

# RULE 4: Allowed processors must be non-empty
deny[msg] {
    count(input.allowed_processors) == 0
    msg := "ALLOWED_PROCESSORS_REQUIRED: allowed_processors must list at least one permitted processor ID"
}

# RULE 5: Free tier processor restriction
deny[msg] {
    input.workspace_tier == "free"
    processor := input.allowed_processors[_]
    processor != "brevio-default"
    msg := sprintf(
        "UNAUTHORIZED_PROCESSOR: processor '%v' not permitted on free tier",
        [processor]
    )
}

# RULE 6: Premium memory types require paid tier
deny[msg] {
    input.workspace_tier == "free"
    premium_types := {"rule", "preference", "procedural"}
    premium_types[input.memory_type]
    msg := sprintf(
        "MEMORY_TYPE_REQUIRES_PAID_TIER: memory_type '%v' requires pro tier or above (current: '%v')",
        [input.memory_type, input.workspace_tier]
    )
}

# RULE 7: Content size limits by tier
deny[msg] {
    max_bytes := {"free": 5000, "pro": 50000, "business": 200000, "enterprise": 1000000}
    limit := max_bytes[input.workspace_tier]
    input.content_byte_size > limit
    msg := sprintf(
        "CONTENT_SIZE_EXCEEDED: content size %v bytes exceeds %v tier limit of %v bytes",
        [input.content_byte_size, input.workspace_tier, limit]
    )
}

# RULE 8: TTL floor for permanent memory types
deny[msg] {
    permanent_types := {"rule", "preference"}
    permanent_types[input.memory_type]
    input.ttl_override_days > 0
    input.ttl_override_days < 365
    msg := sprintf(
        "TTL_BELOW_FLOOR: memory_type '%v' requires minimum TTL of 365 days (requested: %v days)",
        [input.memory_type, input.ttl_override_days]
    )
}

# RULE 9: Sensitive data requires isolated workspace
deny[msg] {
    sensitive_labels := {"sensitive", "regulated"}
    sensitive_labels[input.sensitivity_label]
    non_isolated := {"default", "shared", ""}
    non_isolated[input.workspace_id]
    msg := sprintf(
        "SENSITIVE_DATA_REQUIRES_ISOLATED_WORKSPACE: sensitivity_label '%v' cannot be written to workspace '%v'",
        [input.sensitivity_label, input.workspace_id]
    )
}

package brevio.call_approval

import rego.v1

default allow := false

# Outbound calls require explicit approval
allow if {
    input.approval_status == "approved"
    not input.kill_switch_active
    not blocked_number
    within_rate_limit
    within_allowed_hours
}

allow if {
    input.approval_status == "auto_approved"
    not input.kill_switch_active
    not blocked_number
    within_rate_limit
    within_allowed_hours
    input.risk_level in {"LOW", "MEDIUM"}
}

blocked_number if {
    input.target_number_hash in input.blocklist
}

within_rate_limit if {
    input.calls_in_window < input.max_calls_per_window
}

within_allowed_hours if {
    input.current_hour >= input.allowed_start_hour
    input.current_hour < input.allowed_end_hour
}

# Deny reasons for audit
deny_reason := "KILL_SWITCH_ACTIVE" if {
    input.kill_switch_active
}

deny_reason := "NUMBER_BLOCKED" if {
    blocked_number
}

deny_reason := "RATE_LIMIT_EXCEEDED" if {
    not within_rate_limit
}

deny_reason := "OUTSIDE_ALLOWED_HOURS" if {
    not within_allowed_hours
}

deny_reason := "NOT_APPROVED" if {
    input.approval_status != "approved"
    input.approval_status != "auto_approved"
}

# Transcript-only invariant: raw audio storage is never permitted
deny_audio_storage if {
    input.action == "store_audio"
}

package brevio.v91

# V9.1 OPA rule set (8 rules)

self_modification_gate_deny := {"result": "deny", "reason": "SELF_MODIFICATION_DENIED"} if {
  input.self_modification_action == "deny"
}

self_modification_approval_require := {"result": "require_approval", "reason": "REQUIRE_APPROVAL"} if {
  input.self_modification_action == "require_approval"
}

self_modification_audit_allow := {"result": "allow", "reason": "ALLOW_WITH_AUDIT"} if {
  input.self_modification_action == "allow_with_audit"
}

autonomy_promotion_cap_deny := {"result": "deny", "reason": "PROMOTION_EXCEEDS_SYSTEM_CAP"} if {
  input.requested_level > input.system_cap_level
}

goal_creation_rate_limit_deny := {"result": "deny", "reason": "GOAL_RATE_LIMIT"} if {
  input.goals_created_today >= 20
}

learning_lesson_cap_deny := {"result": "deny", "reason": "LESSON_CAP_REACHED"} if {
  input.active_lessons >= input.max_active_lessons
}

code_context_export_rate_deny := {"result": "deny", "reason": "EXPORT_RATE_LIMIT"} if {
  input.exports_today >= 10
}

daily_capture_uniqueness_skip := {"result": "skip", "reason": "DAILY_CAPTURE_UNIQUENESS"} if {
  input.daily_capture_exists
}

package brevio.v92

# V9.2 OPA rule set (18 rules)

context_budget_gate_deny := {"result": "deny", "reason": "CONTEXT_BUDGET_EXCEEDED"} if {
  input.context_tokens_used > input.context_tokens_budget
}

rag_token_budget_gate_deny := {"result": "deny", "reason": "RAG_BUDGET_EXCEEDED"} if {
  input.rag_tokens_used > input.rag_tokens_budget
}

session_expiry_gate_deny := {"result": "deny", "reason": "SESSION_EXPIRED"} if {
  input.session_expired
}

temporal_constraint_violation_deny := {"result": "deny", "reason": "TEMPORAL_CONSTRAINT_VIOLATION"} if {
  input.temporal_conflict_priority >= 80
}

guardrail_block_override_deny := {"result": "deny", "reason": "GUARDRAIL_BLOCK_ACTIVE"} if {
  input.guardrail_block_active
}

tool_quarantine_gate_deny := {"result": "deny", "reason": "TOOL_QUARANTINED"} if {
  input.tool_quarantined
}

feature_flag_gate_deny := {"result": "deny", "reason": "FEATURE_DISABLED"} if {
  not input.feature_enabled
}

model_tier_cap_deny := {"result": "deny", "reason": "MODEL_TIER_EXCEEDED"} if {
  input.requested_tier > input.max_allowed_tier
}

react_step_limit_terminate := {"result": "terminate", "reason": "MAX_STEPS_REACHED"} if {
  input.react_steps >= input.max_react_steps
}

pii_encryption_gate_deny := {"result": "deny", "reason": "PII_ENCRYPTION_REQUIRED"} if {
  input.pii_encryption_required
  not input.pii_encryption_applied
}

mcp_sandbox_enforcement_deny := {"result": "deny", "reason": "SANDBOX_VIOLATION"} if {
  input.sandbox_violation
}

dsr_sla_warning_escalate := {"result": "escalate", "reason": "DSR_SLA_AT_RISK"} if {
  input.dsr_sla_at_risk
}

event_schema_validation_deny := {"result": "deny", "reason": "EVENT_SCHEMA_INVALID"} if {
  not input.event_schema_valid
}

cache_write_size_limit_deny := {"result": "deny", "reason": "CACHE_ENTRY_TOO_LARGE"} if {
  input.cache_entry_bytes > 1048576
}

conflict_resolution_manual_pause := {"result": "pause", "reason": "CONFLICT_REQUIRES_MANUAL_REVIEW"} if {
  input.conflict_requires_manual_review
}

streaming_first_byte_sla_warn := {"result": "warn", "reason": "FIRST_BYTE_SLA_BREACH"} if {
  input.streaming_first_byte_ms > 500
}

admin_action_audit_allow := {"result": "allow", "reason": "ADMIN_ACTION_AUDITED"} if {
  input.admin_action
}

compliance_evidence_integrity_deny := {"result": "deny", "reason": "EVIDENCE_HASH_MISSING"} if {
  input.evidence_hash_missing
}

package brevio.v10

import rego.v1

default allow := false

# Kill switch gate — highest precedence, non-bypassable
deny_kill_switch if {
    input.kill_switch_active == true
}

# Federation gate
allow if {
    not deny_kill_switch
    input.action == "federation_negotiate"
    input.federation_permission in {"calendar_query", "calendar_write", "routing_negotiate", "task_delegate", "knowledge_share", "status_query"}
    input.peer_trust_score >= 0.5
}

# Wallet gate
allow if {
    not deny_kill_switch
    input.action == "wallet_debit"
    input.balance_usd > input.amount_usd
    input.amount_usd > 0
}

# Call approval gate
allow if {
    not deny_kill_switch
    input.action == "make_call"
    input.call_approved == true
    not input.number_blocked
    input.within_allowed_hours == true
}

deny_call if {
    input.action == "make_call"
    input.number_blocked == true
}

deny_call if {
    input.action == "make_call"
    not input.call_approved
}

# Skills gate
allow if {
    not deny_kill_switch
    input.action == "execute_skill"
    input.skill_enabled == true
    input.sandbox_compliant == true
    input.receipt_valid == true
}

deny_skill if {
    input.action == "execute_skill"
    not input.receipt_valid
}

# DM pairing gate
allow if {
    not deny_kill_switch
    input.action == "delegate_action"
    input.pairing_valid == true
    input.delegation_active == true
}

deny_pairing if {
    input.action == "delegate_action"
    not input.pairing_valid
}

# Sandbox gate
allow if {
    not deny_kill_switch
    input.action == "sandbox_execution"
    input.sandbox_profile in {"strict", "standard", "permissive", "custom"}
    input.within_resource_limits == true
}

# Budget enforcement gate
allow if {
    not deny_kill_switch
    input.action == "budget_check"
    input.daily_cost_usd < input.daily_limit_usd
    input.monthly_cost_usd < input.monthly_limit_usd
}

# Authorization receipt requirement — deny-by-default for side effects
deny_no_receipt if {
    input.has_side_effects == true
    not input.receipt_valid
}

# Admin operations require AdminJWT
allow if {
    not deny_kill_switch
    input.action == "admin_operation"
    input.admin_jwt_valid == true
    input.admin_role in {"super_admin", "workspace_admin", "billing_admin", "support_admin"}
}

package brevio.v10_test

import rego.v1

import data.brevio.v10

test_kill_switch_blocks_all if {
    not v10.allow with input as {
        "kill_switch_active": true,
        "action": "execute_skill",
        "skill_enabled": true,
        "sandbox_compliant": true,
        "receipt_valid": true
    }
}

test_skill_allowed_with_receipt if {
    v10.allow with input as {
        "kill_switch_active": false,
        "action": "execute_skill",
        "skill_enabled": true,
        "sandbox_compliant": true,
        "receipt_valid": true
    }
}

test_skill_denied_without_receipt if {
    not v10.allow with input as {
        "kill_switch_active": false,
        "action": "execute_skill",
        "skill_enabled": true,
        "sandbox_compliant": true,
        "receipt_valid": false
    }
}

test_call_allowed_when_approved if {
    v10.allow with input as {
        "kill_switch_active": false,
        "action": "make_call",
        "call_approved": true,
        "number_blocked": false,
        "within_allowed_hours": true
    }
}

test_call_denied_when_blocked if {
    not v10.allow with input as {
        "kill_switch_active": false,
        "action": "make_call",
        "call_approved": true,
        "number_blocked": true,
        "within_allowed_hours": true
    }
}

test_federation_allowed if {
    v10.allow with input as {
        "kill_switch_active": false,
        "action": "federation_negotiate",
        "federation_permission": "calendar_query",
        "peer_trust_score": 0.8
    }
}

test_federation_denied_low_trust if {
    not v10.allow with input as {
        "kill_switch_active": false,
        "action": "federation_negotiate",
        "federation_permission": "calendar_query",
        "peer_trust_score": 0.3
    }
}

test_admin_allowed_with_jwt if {
    v10.allow with input as {
        "kill_switch_active": false,
        "action": "admin_operation",
        "admin_jwt_valid": true,
        "admin_role": "super_admin"
    }
}

test_admin_denied_without_jwt if {
    not v10.allow with input as {
        "kill_switch_active": false,
        "action": "admin_operation",
        "admin_jwt_valid": false,
        "admin_role": "super_admin"
    }
}

test_budget_allowed_within_limits if {
    v10.allow with input as {
        "kill_switch_active": false,
        "action": "budget_check",
        "daily_cost_usd": 50,
        "daily_limit_usd": 100,
        "monthly_cost_usd": 1000,
        "monthly_limit_usd": 3000
    }
}

test_budget_denied_over_daily if {
    not v10.allow with input as {
        "kill_switch_active": false,
        "action": "budget_check",
        "daily_cost_usd": 150,
        "daily_limit_usd": 100,
        "monthly_cost_usd": 1000,
        "monthly_limit_usd": 3000
    }
}

test_no_receipt_denies_side_effects if {
    v10.deny_no_receipt with input as {
        "has_side_effects": true,
        "receipt_valid": false
    }
}

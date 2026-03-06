package brevio.routing_gate

default allow = true

deny[msg] {
  input.estimated_cost_cents > input.max_cost_per_request_cents
  msg := "ROUTING_COST_EXCEEDS_LIMIT"
}

deny[msg] {
  input.provider_blocked
  msg := "ROUTING_PROVIDER_BLOCKED"
}

deny[msg] {
  input.model_blocked
  msg := "ROUTING_MODEL_BLOCKED"
}

deny[msg] {
  input.daily_cost_cents > input.daily_budget_cents
  msg := "ROUTING_DAILY_BUDGET_EXCEEDED"
}

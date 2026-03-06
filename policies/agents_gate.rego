package brevio.agents_gate

default allow = false

allow {
  input.user_tier == "enterprise"
  input.skill_id == "agents.supervisor"
}

allow {
  input.user_tier != "free"
  input.skill_id == "agents.worker"
}

allow {
  input.user_tier != "free"
  input.skill_id == "agents.evaluator"
}

allow {
  input.user_tier != "free"
  input.skill_id == "agents.planner"
}

allow {
  input.user_tier != "free"
  input.skill_id == "agents.tool_executor"
}

deny[msg] {
  input.skill_id == "agents.supervisor"
  input.concurrent_agents > 10
  msg := "AGENTS_MAX_CONCURRENT_EXCEEDED"
}

deny[msg] {
  input.skill_id == "agents.worker"
  input.iteration_count > input.max_iterations
  msg := "AGENTS_MAX_ITERATIONS_EXCEEDED"
}

deny[msg] {
  input.delegation_depth > 3
  msg := "AGENTS_MAX_DELEGATION_DEPTH"
}

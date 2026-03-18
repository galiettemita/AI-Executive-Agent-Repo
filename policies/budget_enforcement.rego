package brevio.budget

deny[msg] {
  input.budget_exhausted
  msg := "BUDGET_CALLS_EXHAUSTED"
}

# Cap total thinking tokens per workflow at 131,072
deny[msg] {
  input.thinking_tokens_used + input.thinking_tokens_requested > 131072
  msg := sprintf("THINKING_TOKEN_CAP_EXCEEDED: used=%d requested=%d cap=131072",
      [input.thinking_tokens_used, input.thinking_tokens_requested])
}

package brevio.budget

deny[msg] {
  input.budget_exhausted
  msg := "BUDGET_CALLS_EXHAUSTED"
}

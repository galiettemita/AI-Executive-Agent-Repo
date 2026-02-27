package brevio.rate_limit

deny[msg] {
  input.rate_limited
  msg := "RATE_LIMIT_EXCEEDED"
}

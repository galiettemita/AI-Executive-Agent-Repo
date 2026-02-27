package brevio.domain_constraints

deny[msg] {
  input.domain == "health"
  input.autonomy_level != "A0"
  msg := "HEALTH_DOMAIN_AUTONOMY_INVALID"
}

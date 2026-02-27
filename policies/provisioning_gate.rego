package brevio.provisioning_gate

deny[msg] {
  input.provisioning_risk_exceeds_max
  msg := "PROVISIONING_RISK_EXCEEDED"
}

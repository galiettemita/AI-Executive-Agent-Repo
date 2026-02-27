package brevio.firewall

deny[msg] {
  not input.firewall_allowed
  msg := "FIREWALL_BLOCKED"
}

package brevio.memory_write_gate

deny[msg] {
  input.memory_write_blocked
  msg := "MEMORY_WRITE_BLOCKED"
}

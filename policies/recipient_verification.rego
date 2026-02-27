package brevio.recipient_verification

deny[msg] {
  input.recipient_unverified
  msg := "RECIPIENT_UNVERIFIED"
}

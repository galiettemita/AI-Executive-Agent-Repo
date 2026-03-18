package brevio.autonomy

default allow = false

allow {
  input.autonomy_level == "A4"
}

# Wallet writes ALWAYS require explicit user approval, regardless of autonomy tier.
deny[msg] {
  input.tool_key == "wallet.create_payment_request"
  not input.user_approved
  msg := "WALLET_APPROVAL_REQUIRED: wallet writes require explicit user approval"
}

# Wallet writes are blocked at autonomy levels below A3.
deny[msg] {
  input.tool_key == "wallet.create_payment_request"
  input.autonomy_tier < 3
  msg := "WALLET_LOW_AUTONOMY: wallet writes blocked below A3"
}

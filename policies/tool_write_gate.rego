package brevio.tool_write_gate

default require_approval = false

require_approval {
  input.is_write
  input.autonomy_level == "A1"
}

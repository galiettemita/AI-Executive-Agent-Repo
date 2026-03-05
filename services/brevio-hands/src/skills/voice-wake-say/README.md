# voice-wake-say

Gateway-plane local fallback TTS skill using macOS `say` command semantics.

## Supported action

- Generates local-synthesis command metadata for low-latency voice responses.

## Notes

- Enforces Gateway latency budget contract (`latency_budget_ms = 500`).
- No remote API requirement.

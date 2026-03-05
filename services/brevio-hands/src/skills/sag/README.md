# sag

Gateway-plane premium text-to-speech adapter (ElevenLabs-style output contract).

## Supported action

- Converts text to voice-ready output metadata for channel egress.

## Notes

- Enforces Gateway latency budget contract (`latency_budget_ms = 3000`).
- Deterministic output metadata until external provider credentials are wired.

# vocal-chat

Gateway-plane end-to-end voice pipeline skill (STT + response + TTS contract).

## Supported action

- Accepts inbound audio and returns transcript plus synthesized reply metadata.

## Notes

- Encodes round-trip gateway latency contract (`latency_budget_ms = 5000`).
- Deterministic routing between `asr` and `gemini-stt` fixtures by clip duration.

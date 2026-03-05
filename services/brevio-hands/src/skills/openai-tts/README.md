# openai-tts

Gateway-plane text-to-speech skill using the OpenAI voice profile contract.

## Supported action

- Converts text responses into audio metadata for channel delivery.

## Notes

- Enforces Gateway latency budget contract (`latency_budget_ms = 2000`).
- Output is deterministic fixture metadata until live provider wiring is enabled.

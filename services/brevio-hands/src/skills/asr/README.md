# asr

Gateway-plane speech-to-text skill for inbound voice message normalization.

## Supported action

- Transcribes HTTPS audio input into text before Brain-plane classification.

## Notes

- Enforces gateway latency budget contract (`latency_budget_ms = 3000`).
- Deterministic transcript fixtures are used pending live provider wiring.

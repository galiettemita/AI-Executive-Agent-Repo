# gemini-stt

Gateway-plane premium STT skill with optional speaker labels.

## Supported action

- Transcribes HTTPS audio payloads before Brain-plane routing.

## Notes

- Enforces gateway latency budget contract (`latency_budget_ms = 5000`).
- Returns structured speaker segments for downstream meeting decomposition.

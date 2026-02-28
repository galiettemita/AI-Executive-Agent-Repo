# Load Tests

Run k6 interactive-turn baseline:

```bash
k6 run evals/load/k6_interactive_turn.js
```

Run load-shedding tier probes (D0-D5):

```bash
k6 run evals/load/k6_load_shedding.js
```

Run streaming first-byte SLA probes (V9.2, <=500ms P95):

```bash
k6 run evals/load/k6_streaming_first_byte.js
```

Environment variable:
- `BASE_URL` (optional): target control/gateway endpoint base URL.
- `WEBHOOK_SECRET` (optional): HMAC secret used to sign `X-Signature` for webhook admission tests.
- `SHED_TIER` (optional): override scenario tier when running a single load-shedding scenario.

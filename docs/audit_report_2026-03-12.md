AUDIT EXTRACT (AUTHORITATIVE MUST-FIX)
- Silent tool execution success when HandsExecutor is nil:
  internal/temporal/activities.go ExecuteToolActivity returns Success=true
  without execution.
- Silent outbox dispatch success when OutboxDispatcher is nil:
  internal/temporal/activities.go DispatchOutboxEntryActivity can mark
  entries dispatched even when dispatcher is nil.
- Temporal worker does not wire HandsExecutor nor OutboxDispatcher:
  cmd/temporal-worker/main.go creates ActivityDeps but does not set
  deps.HandsExecutor nor deps.OutboxDispatcher.
- Hard-coded executor HMAC signing key default:
  cmd/executor/main.go uses a default HMAC key when env HMAC_KEY is missing.
- Canvas WebSocket origin check allows all origins:
  internal/canvas/service.go uses CheckOrigin that always returns true.
- Connector seed includes placeholder MCP URLs:
  internal/connectors/seeds/connectors.yaml includes unconfigured.local placeholder.
- Stub deployable services returning fabricated JSON:
  internal/agents/service.go
  internal/memorysvc/service.go
  internal/router/service.go
  internal/cron/service.go
  internal/browser/service.go
  internal/marketing/service.go
  and their cmd/* entrypoints.
- Observability is stub/no-op:
  internal/observability/otel.go indicates production intent but does not
  implement real exporters and shutdown.
- Placeholder markers existed and have been eliminated repo-wide
  per the banned-token policy.

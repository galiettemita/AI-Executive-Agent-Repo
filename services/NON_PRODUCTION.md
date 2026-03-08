# NON-PRODUCTION — Quarantined TypeScript Services

The TypeScript services in this directory are **quarantined duplicates** of the
authoritative Go control-plane services located under `cmd/` and `internal/`.

They exist for historical reference only and **must not be deployed to
production**.

## Authoritative source of truth

| Concern | Location |
|---------|----------|
| Go services (production) | `cmd/` + `internal/` |
| Infrastructure | `infra/terraform/` |
| CI / CD | `.github/workflows/ci.yml` |

## CI enforcement

The companion file `.ci-quarantine.yaml` in this directory is read by CI to
block any attempt to build, push, or deploy these TS services to a production
environment.

If you need to promote a capability currently only in a TS service, port it to
the Go codebase first, then remove the quarantine entry.

# Brevio Implementation Prompt — Summary

## Scope

Transform the AI Executive Agent repository into the reconciled production target state implementing all 17 mandatory blueprint bodies.

## Binding Decisions (D1–D19)

All binding decisions are documented in DECISIONS.md. Key constraints:

1. Go services for all cloud production planes
2. Temporal-only orchestration (no in-process simulators)
3. Control is sole PDP with durable authorization receipts
4. workspace_id RLS enforcement on all tables
5. UUIDv7 for all primary keys
6. Forward-only migrations
7. FNV-1a deterministic jitter
8. OpenClaw Hands runtime with versioned contract
9. PostgreSQL-only production persistence (S1)
10. Embedding-based similarity only (S3)

## Anti-Stub Constraints (S1–S3)

- **S1**: No in-memory authoritative state in production
- **S2**: Provider contract tests with real HTTP roundtrips
- **S3**: No lexical heuristics substituting embeddings

## CI/CD Gates (A–H)

All gates enforced via brevioctl verify commands:
- A: Blueprint coverage + graph + traceability
- B: Architecture coherence + S1
- C: Schema + contract closure
- D: Policy closure + receipts + secrets
- E: Temporal-only + replay + determinism
- F: Build/test + S2 + S3
- G: Deployability + infra
- H: Documentation + exports

## Verification Environment

Pinned in docker-compose.verify.yml:
- Go 1.23.x, Node 24.x, PostgreSQL 16 + pgvector
- Temporal 1.25.2, OTel Collector 0.96.0
- pnpm 9.15.4 via Corepack

## Artifact Outputs

- ARCHITECTURE.md, DECISIONS.md, RUNBOOK.md
- api/openapi/v10.yaml
- docker-compose.verify.yml
- reports/PROGRESS.json
- reports/requirements_graph.json
- reports/traceability_matrix.json
- reports/blueprints/ (manifest, line index, extract inventory, coverage matrix)
- reports/exports/ (this document)

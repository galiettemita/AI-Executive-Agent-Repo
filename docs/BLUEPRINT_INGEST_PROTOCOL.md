# Blueprint Ingestion Protocol

## Purpose

This document defines the deterministic rules for ingesting blueprint documents into the BREVIO repository. It ensures that blueprint authority is derived from content identity (sha256), not from filenames, titles, or directory placement.

## Authoritative Manifest

The single source of truth for which blueprints govern the platform is:

```
docs/BLUEPRINT_MANIFEST.json
```

Any blueprint not listed in the manifest's `blueprints` array is non-authoritative.

## Non-Authoritative Identifiers

The following are explicitly **non-authoritative** and must not be used to determine blueprint identity or precedence:

- Document filenames (may be renamed, abbreviated, or inconsistent across sources)
- Document titles embedded in OOXML metadata
- Directory names containing blueprint files
- ZIP archive names used for transport

## Authoritative Identifiers

Blueprint identity is determined exclusively by:

1. **`blueprint_id`** — Stable identifier (BP06, BP07, BP08, BP09, BP11, BP16, BP17)
2. **`sha256`** — Content hash of the `.docx` file at time of ingestion
3. **`authority_rank`** — Numeric precedence (1 = highest authority for conflicts)

## Ingestion Rules

### Rule 1: Hash Verification

Before referencing any blueprint, compute its sha256 and verify it matches the manifest:

```bash
shasum -a 256 extracted-blueprints/<filename>.docx
```

If the hash does not match, the file has been modified since ingestion and must not be treated as authoritative until the manifest is updated.

### Rule 2: Conflict Resolution

When blueprints conflict on a requirement:

1. Higher `authority_rank` (lower number) wins
2. Later version scopes refine — not replace — earlier scopes unless explicitly stated
3. Supplemental documents inform but do not override mandatory blueprints

### Rule 3: Requirement Extraction

Requirements are extracted from blueprint content, not summaries. Each requirement must reference its `blueprint_id` source. Requirements without traceable blueprint sources are rejected.

### Rule 4: Completeness Assertion

The manifest must list exactly 7 mandatory blueprints:

| ID   | Version Scope    |
|------|-----------------|
| BP06 | V9              |
| BP07 | V9.1            |
| BP08 | V9.2            |
| BP09 | V10             |
| BP11 | V10.1           |
| BP16 | OpenClaw-v1.0   |
| BP17 | OpenClaw-v1.0.1 |

Contract tests validate this count and the presence of all 7 IDs.

### Rule 5: Immutability After Ingestion

Once a blueprint is ingested and its hash recorded in the manifest, the source `.docx` file must not be modified. If a blueprint requires revision, a new blueprint_id and entry must be created.

## Contract Enforcement

The following contract tests validate the manifest:

- `TestBlueprintManifestCompleteness` — Asserts exactly 7 mandatory blueprints exist
- `TestBlueprintManifestHashIntegrity` — Verifies sha256 matches for all listed files
- `TestCIWorkflowAuthority` — Asserts the authoritative CI workflow exists and the quarantined one is absent from active triggers

## Supplemental Documents

Documents listed under `non_mandatory_supplemental` in the manifest are reference material. They inform implementation but do not independently define requirements. Their content is subsumed by the mandatory blueprints they feed into.

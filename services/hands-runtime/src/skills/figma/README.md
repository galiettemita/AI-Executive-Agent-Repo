# figma

Hands-plane adapter for Figma analysis, export, and accessibility auditing.

## Supported actions

- `analyze_file`
- `export_asset`
- `audit_accessibility`

## Notes

- Requires `file_key` for all actions and `node_id` for export.
- Emits deterministic findings and export URLs for CI-safe behavior.

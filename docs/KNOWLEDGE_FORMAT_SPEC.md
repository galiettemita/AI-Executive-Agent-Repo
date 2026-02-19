# Knowledge Format Spec

## Overview
Knowledge files are versioned Markdown documents stored per user in the `knowledge_files` table. They power personalization, routing, and proactive behavior. Each write creates a new version so we always have a history of changes.

## Core Files
- `USER.md`: Identity, profile, and stable preferences.
- `MEMORY.md`: Durable facts and long-lived preferences.
- `HEARTBEAT.md`: Current priorities, goals, and check-in focus items.
- `TEAM.md`: Team context, stakeholders, and working norms.
- `WORKFLOWS.md`: Reusable procedures and routines.
- `TOOLS.md`: Connected services and tool availability.
- `AGENTS.md`: Delegation and agent guidance.

## Format Rules
- Use Markdown with clear section headers (H1/H2).
- Prefer short bullet lists for facts and preferences.
- Keep sensitive data minimal; store only what is needed for personalization.
- Avoid free-form narrative for durable facts; prefer structured bullets.
- Keep HEARTBEAT items concise and actionable.

## Update Workflow
- All updates should be written via `PUT /api/v1/knowledge/{file_path}`.
- Always create a new version; do not mutate history.
- Include a `metadata` payload for provenance (for example: `source`, `confidence`, `tags`).

## Metadata Guidance
Recommended keys in `metadata`:
- `source`: where the update came from (chat, delegation, import).
- `confidence`: a number or label (low/med/high).
- `tags`: list of topical tags.
- `reviewed_at`: ISO timestamp if validated by user.

## Example
```
# USER.md
## BASICS
- Name: Sam
- Timezone: America/Los_Angeles

## WORK
- Role: VP Operations
- Company: ExampleCo

## OPERATIONS
- Daily patterns: Morning standup at 9am
- Meeting preferences: 30 min blocks
```

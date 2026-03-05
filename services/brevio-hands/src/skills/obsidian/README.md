# obsidian

Notes/PKM adapter for obsidian operations.

- Plane: hands
- External API target: obsidian provider bridge
- Auth: provider-specific (OAuth/local app permissions)

## Input

- action (list, create, search, update)
- note fields (note_id, title, content, query)

## Output

- provider: obsidian
- action-specific note_id and notes[]

## Brevio use case

Save notes and search prior knowledge entries with deterministic routing.

# gkeep

Notes/PKM adapter for gkeep operations.

- Plane: hands
- External API target: gkeep provider bridge
- Auth: provider-specific (OAuth/local app permissions)

## Input

- action (list, create, search, update)
- note fields (note_id, title, content, query)

## Output

- provider: gkeep
- action-specific note_id and notes[]

## Brevio use case

Save notes and search prior knowledge entries with deterministic routing.

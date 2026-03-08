# bear-notes

Notes/PKM adapter for bear-notes operations.

- Plane: hands
- External API target: bear-notes provider bridge
- Auth: provider-specific (OAuth/local app permissions)

## Input

- action (list, create, search, update)
- note fields (note_id, title, content, query)

## Output

- provider: bear-notes
- action-specific note_id and notes[]

## Brevio use case

Save notes and search prior knowledge entries with deterministic routing.

# reflect

Notes/PKM adapter for reflect operations.

- Plane: hands
- External API target: reflect provider bridge
- Auth: provider-specific (OAuth/local app permissions)

## Input

- action (list, create, search, update)
- note fields (note_id, title, content, query)

## Output

- provider: reflect
- action-specific note_id and notes[]

## Brevio use case

Save notes and search prior knowledge entries with deterministic routing.

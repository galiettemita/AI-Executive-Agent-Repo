# apple-notes-skill

Notes/PKM adapter for apple-notes-skill operations.

- Plane: hands
- External API target: apple-notes-skill provider bridge
- Auth: provider-specific (OAuth/local app permissions)

## Input

- action (list, create, search, update)
- note fields (note_id, title, content, query)

## Output

- provider: apple-notes-skill
- action-specific note_id and notes[]

## Brevio use case

Save notes and search prior knowledge entries with deterministic routing.

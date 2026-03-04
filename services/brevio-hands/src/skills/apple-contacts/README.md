# apple-contacts

Local Apple Contacts search adapter.

- Plane: `hands`
- External API target: Contacts.app local bridge (`contacts-cli`) in production local-mac mode
- Auth: macOS local permissions via Edge Agent / host runtime

## Input

- `query` (required)

## Output

- `provider`: `apple-contacts-local`
- `contacts[]` with `name`, `phone`, `email`

## Brevio use case

"What's Sarah's phone number?" -> resolves contact details from local Apple Contacts.

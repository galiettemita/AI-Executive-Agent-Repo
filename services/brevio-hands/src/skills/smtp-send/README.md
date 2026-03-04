# smtp-send

Email send adapter over SMTP semantics.

- Plane: `hands`
- External API target: SMTP provider endpoint
- Auth: SMTP credentials / provider relay credentials

## Safety constraints

- Requires explicit `confirmed=true` before `sent=true`.
- Intended to enforce recipient/content confirmation before delivery.

## Brevio use case

"Email my landlord about the dishwasher" -> draft + confirmation + send.

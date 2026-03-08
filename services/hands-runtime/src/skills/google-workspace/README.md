# google-workspace

Google Workspace unified mail/calendar/drive adapter.

- Plane: `hands`
- External API target: Gmail + Calendar + Drive APIs
- Auth: OAuth2 scopes `gmail.modify`, `calendar`, `drive`

## Input

- `action` (`gmail_list`, `gmail_send`, `calendar_list`, `drive_search`)
- `query` for drive search
- `to`, `subject`, `body`, `confirmed` for mail send

## Output

- `provider`: `google-workspace`
- action-specific `mails[]`, `events[]`, `files[]`, `message_id`, `confirmation_required`

## Brevio use case

"Send this to my team" and "What's on my Google calendar today?" with one routed skill.

# outlook

Microsoft Outlook mail/calendar adapter.

- Plane: `hands`
- External API target: Microsoft Graph Outlook endpoints
- Auth: OAuth2 scopes `Mail.ReadWrite`, `Calendars.ReadWrite`, `offline_access`

## Input

- `action` (`inbox_list`, `send`, `calendar_list`)
- send fields: `to[]`, `subject`, `body`, `confirmed`

## Output

- `provider`: `outlook`
- `mails[]` for inbox
- `events[]` for calendar
- `message_id` or `confirmation_required` for send

## Brevio use case

"Reply from Outlook" and "Show my Outlook calendar" with unified action routing.

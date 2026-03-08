# imap-email

IMAP email adapter for generic mailbox listing/search plus confirmation-gated send.

## Auth
- IMAP credentials with read/send privileges.

## Input
- `action`: `list`, `search`, `send`
- `mailbox` optional mailbox name
- `query` required for `search`
- `to`, `subject`, `body`, `confirmed` for `send`

## Output
- `provider`: `imap-email`
- action echo, mailbox, optional `messages[]`, optional `sent`

## Notes
- Send is blocked unless `confirmed: true`.

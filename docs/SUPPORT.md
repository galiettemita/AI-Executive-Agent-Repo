# Support & Operations Guide

## Support Channels
- Primary: WhatsApp (user-facing)
- Internal: Slack alerting channel (ops)

## Common Support Flows
- **User can’t connect Google/Microsoft**
  - Verify OAuth client id/secret and redirect URIs match environment.
  - Confirm consent stored in `user_consents`.
- **Messages not sending**
  - Check `ENABLE_MESSAGING=1`.
  - Verify WhatsApp credentials and webhook receipt logs.
- **Email drafts not sending**
  - Confirm provider tokens exist for the user.
  - Check email draft status in `email_drafts`.

## Escalation
- P1 (service down): page on-call, post incident in ops channel.
- P2 (partial outage): create incident ticket, triage within 1 hour.

## Data Requests
- Use GDPR endpoints or admin scripts to export or delete user data.
- Ensure deletions include `file_assets`, `photo_assets`, and `usage_events`.

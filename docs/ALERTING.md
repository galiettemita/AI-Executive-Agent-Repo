# Alerting and On-Call

## Primary
- Sentry for error monitoring and alerting

## On-Call Routing
- Use Sentry integrations to route alerts to PagerDuty or Opsgenie

## Optional
- Slack webhook alerts for high-priority events

## Required Inputs
- SENTRY_DSN
- METRICS_TOKEN (if exposing /metrics)
- ALERTING_PROVIDER (sentry | pagerduty | slack)

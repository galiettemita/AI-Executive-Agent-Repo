# Phase 4 Partner Fallback Matrix

## Purpose
Define approved fallback choices for integrations that may be blocked by partner approval timing.

## Fallbacks
| Target Integration | Primary Choice | Fallback Choice | Trigger Condition |
|---|---|---|---|
| Zoom meetings | Zoom Marketplace OAuth app | Zoom PAT + server-side token vault | Marketplace app pending or rejected |
| Instacart checkout | Instacart Connect | Amazon Fresh or DoorDash MCP adapter | Instacart approval blocked |
| Canva design generation | Canva Connect | Figma template export workflow | Canva partner onboarding delayed |
| Booking.com lodging | Booking.com Demand API | Booking affiliate flow with explicit user confirmation | Demand API access not granted |

## Guardrails
- Keep approval gates unchanged for any booking/payment write operation.
- Preserve provenance tags (`mcp_result`) and risk scoring.
- Require explicit user confirmation before final checkout/booking.

# Research API Guide

## Overview
Research jobs run scheduled or on-demand analyses and can deliver results to a chosen channel.

## Endpoints
- `GET /api/v1/research?user_id=...`
- `POST /api/v1/research`
- `GET /api/v1/research/{research_id}?user_id=...`
- `PUT /api/v1/research/{research_id}?user_id=...`
- `DELETE /api/v1/research/{research_id}?user_id=...`
- `POST /api/v1/research/{research_id}/run?user_id=...`

## Create Payload
Required:
- `user_id`
- `title`
- `query`
Optional:
- `sources` (list)
- `schedule` (cron or interval string)
- `status` (active/paused)
- `delivery_channel` (whatsapp/email/slack)
- `delivery_format` (summary/report)
- `max_cost_per_run`

## Operational Notes
- Scheduled jobs are picked up by the research scheduler.
- Deliveries are recorded in analytics and notification queues.
- Use opt-in preferences for anonymized insight sharing.

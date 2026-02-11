# Executive AI Agent API Docs (Summary)

This is a high-level API reference. The canonical schema is the generated OpenAPI document in `openapi.json`.

## Authentication
- All endpoints expect `user_id` for now (WhatsApp phone number or internal user id).
- Webhooks are verified when `ENFORCE_WEBHOOK_SIGNATURES=1`.

## Core Endpoints
- `GET /health/ready` readiness probe
- `POST /chat` conversational entry point
- `POST /assist` structured task execution

## Messaging
- `POST /messages/send` queue outbound messages (WhatsApp/SMS)
- `POST /webhooks/whatsapp` inbound WhatsApp messages
- `POST /webhooks/sms/status` delivery receipts

## Email
- `GET /email/intelligence/summary` inbox summarization
- `POST /email/intelligence/reply/draft` create draft
- `POST /email/intelligence/reply/send` send draft
- `GET /email/intelligence/monitoring/configs` list monitoring configs
- `POST /email/intelligence/monitoring/configs` upsert monitoring config
- `POST /email/intelligence/monitoring/run` trigger monitoring
- `GET /email/intelligence/monitoring/alerts` list alerts
- `POST /email/intelligence/monitoring/test` create a test alert (non-prod unless enabled)
- `POST /admin/email/connect` connect iCloud or Yahoo (app-specific password)
- `GET /admin/email/status` check iCloud/Yahoo status
- `POST /admin/email/disconnect` disconnect iCloud/Yahoo
- `GET /admin/fitbit/connect` get Fitbit OAuth URL
- `GET /admin/fitbit/callback` Fitbit OAuth callback
- `GET /admin/fitbit/status` Fitbit connection status
- `POST /admin/fitbit/disconnect` disconnect Fitbit

## Calendar
- `GET /calendar/intelligence/meeting-prep` briefing
- `POST /calendar/intelligence/followup` follow-up email/tasks

## Files & Photos
- `POST /files/upload`
- `GET /files/search?semantic=true`
- `POST /photos/upload`
- `GET /photos/search?semantic=true`

## Wardrobe
- `POST /wardrobe/items` create wardrobe item
- `GET /wardrobe/items` list wardrobe items
- `GET /wardrobe/items/{item_id}` get wardrobe item
- `PATCH /wardrobe/items/{item_id}` update wardrobe item
- `DELETE /wardrobe/items/{item_id}` delete wardrobe item
- `GET /wardrobe/items/{item_id}/photos` list item photos
- `POST /wardrobe/items/{item_id}/photos` attach existing photos
- `POST /wardrobe/items/{item_id}/photos/upload` upload and attach a photo
- `DELETE /wardrobe/items/{item_id}/photos/{photo_id}` detach a photo
- `POST /wardrobe/items/{item_id}/wear` log wear event
- `GET /wardrobe/items/{item_id}/wear` list wear events
- `GET /wardrobe/stats` wear frequency stats
- `GET /wardrobe/rotation` rotation recommendations (optional notify)
- `GET /wardrobe/suggestions` outfit suggestions (weather + calendar)
- `GET /wardrobe/recommendations` shopping recommendations

## Gifts
- `POST /gifts/occasions` create occasion
- `GET /gifts/occasions` list occasions
- `PATCH /gifts/occasions/{occasion_id}` update occasion
- `DELETE /gifts/occasions/{occasion_id}` delete occasion
- `POST /gifts/ideas` create gift idea
- `GET /gifts/ideas` list gift ideas
- `PATCH /gifts/ideas/{idea_id}` update gift idea
- `DELETE /gifts/ideas/{idea_id}` delete gift idea
- `POST /gifts/recommendations` gift recommendations (SerpAPI)
- `POST /gifts/thank-you` generate thank-you note draft
- `POST /gifts/ideas/{idea_id}/proposal` create gift purchase proposal
- `POST /gifts/reminders/run` queue gift reminders for user
- `POST /gifts/retailers` add retailer allowlist entry
- `GET /gifts/retailers` list retailer allowlist entries
- `PATCH /gifts/retailers/{retailer_id}` update retailer allowlist entry
- `DELETE /gifts/retailers/{retailer_id}` delete retailer allowlist entry
- `POST /gifts/orders` create gift order with approval flow
- `GET /gifts/orders` list gift orders
- `GET /gifts/orders/{order_id}` get gift order
- `PATCH /gifts/orders/{order_id}` update gift order status/tracking
- `POST /gifts/orders/{order_id}/authorize` authorize payment method for order
- `POST /gifts/orders/{order_id}/events` add order status event
- `GET /gifts/orders/{order_id}/events` list order status events
- `POST /gifts/orders/{order_id}/refund` mark gift order refund

## Relationships
- `POST /relationships/profiles` create or update a relationship profile (contact required)
- `GET /relationships/profiles` list relationship profiles
- `PATCH /relationships/profiles/{profile_id}` update relationship profile
- `POST /relationships/interactions` log a relationship interaction
- `GET /relationships/suggestions` list reach‑out suggestions (due or all)
- `POST /relationships/reminders/run` queue reach‑out reminders for user

## Fitness & Nutrition
- `POST /fitness/workouts` create workout log
- `GET /fitness/workouts` list workout logs
- `PATCH /fitness/workouts/{workout_id}` update workout log
- `DELETE /fitness/workouts/{workout_id}` delete workout log
- `POST /fitness/nutrition/logs` create nutrition log
- `GET /fitness/nutrition/logs` list nutrition logs
- `PATCH /fitness/nutrition/logs/{log_id}` update nutrition log
- `DELETE /fitness/nutrition/logs/{log_id}` delete nutrition log
- `POST /fitness/meal-plans` create meal plan
- `GET /fitness/meal-plans` list meal plans
- `PATCH /fitness/meal-plans/{plan_id}` update meal plan
- `DELETE /fitness/meal-plans/{plan_id}` delete meal plan
- `GET /fitness/suggestions/workouts` workout suggestions
- `GET /fitness/suggestions/meals` meal plan suggestions
- `GET /fitness/steps` daily steps (requires Fitbit connection)

## Entertainment
- `POST /entertainment/items` create content item
- `GET /entertainment/items` list content items
- `PATCH /entertainment/items/{item_id}` update content item
- `DELETE /entertainment/items/{item_id}` delete content item
- `POST /entertainment/consumption` log a consumption event
- `GET /entertainment/consumption` list consumption events
- `POST /entertainment/recommendations` get content recommendations (SerpAPI)
- `POST /entertainment/events/discover` discover events (SerpAPI)
- `POST /entertainment/events` create event
- `GET /entertainment/events` list events
- `PATCH /entertainment/events/{event_id}` update event
- `DELETE /entertainment/events/{event_id}` delete event
- `POST /entertainment/events/{event_id}/proposal` create ticket booking proposal
- `GET /entertainment/events/bookings` list event bookings
- `PATCH /entertainment/events/bookings/{booking_id}` update event booking

## Learning
- `POST /learning/language/goals` create/update language goal
- `GET /learning/language/goals` list language goals
- `PATCH /learning/language/goals/{goal_id}` update language goal
- `DELETE /learning/language/goals/{goal_id}` delete language goal
- `POST /learning/language/sessions` log language practice session
- `GET /learning/language/sessions` list language practice sessions
- `GET /learning/language/progress` language progress summary + streaks
- `POST /learning/resources` create learning resource
- `GET /learning/resources` list learning resources
- `PATCH /learning/resources/{resource_id}` update learning resource
- `DELETE /learning/resources/{resource_id}` delete learning resource
- `POST /learning/resources/recommendations` recommend learning resources (SerpAPI)
- `POST /learning/schedule` create learning schedule entry
- `GET /learning/schedule` list learning schedule entries
- `PATCH /learning/schedule/{schedule_id}` update learning schedule entry
- `DELETE /learning/schedule/{schedule_id}` delete learning schedule entry

## Analytics
- `GET /analytics/events`
- `GET /analytics/summary`

## Beta Access
- `POST /admin/beta/testers` add or update a beta tester
- `GET /admin/beta/testers` list beta testers
- `DELETE /admin/beta/testers/{tester_id}` remove beta tester
- `POST /admin/beta/testers/bulk` bulk add/update testers
- `GET /admin/beta/summary` beta allowlist summary

## Billing
- `POST /billing/checkout`
- `POST /webhooks/stripe`

## Monitoring
- `POST /monitoring/trigger/price-check`
- `POST /monitoring/trigger/send-notifications`
- `POST /monitoring/trigger/email-monitoring`

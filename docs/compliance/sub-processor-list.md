# Sub-Processor List

## Core Infrastructure
| Processor | Function | Data Categories | DPA Status |
|---|---|---|---|
| AWS | Hosting, DB, cache, object storage, secrets | All platform data | Active |
| Grafana Cloud | Observability | Logs/metrics/traces metadata | Active |
| PagerDuty | Incident alerting | Operational metadata | Active |

## AI Providers
| Processor | Function | Data Categories | DPA Status |
|---|---|---|---|
| Anthropic | Primary LLM inference | Prompt/context payloads | Active |
| OpenAI | Fallback LLM + TTS | Prompt/context and TTS text | Active |

## Connector/API Providers (Representative)
Google, Microsoft, Spotify, Notion, Todoist, ClickUp, Slack, Reddit, Bluesky, Tavily, Exa, Firecrawl, SerpAPI, Perplexity, Kagi, Brave, AviationStack, 17TRACK, TMDB, Plaid, YNAB, Monarch, Dexcom, Withings, Home Assistant, Fal.ai, ElevenLabs.

For each connector:
- Data is scoped to user authorization.
- Credentials are stored encrypted.
- DPA/legal review status tracked in vendor registry.

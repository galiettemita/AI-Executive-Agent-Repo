BEGIN;

WITH seed AS (
  SELECT unnest(ARRAY[
    'shopping-expert','buy-anything','personal-shopper','grocery-list','marketplace','clawringhouse','recipe-to-list'
  ]) AS id, 'shopping_ecommerce'::text AS category
  UNION ALL
  SELECT unnest(ARRAY[
    'google-maps','flight-tracker','aviationstack-flight-tracker','aerobase-skill','parcel-package-tracking','track17','post-at','spots','local-places','goplaces','swissweather'
  ]) AS id, 'transportation'::text AS category
  UNION ALL
  SELECT unnest(ARRAY[
    'google-calendar','apple-remind-me','daily-rhythm','meeting-autopilot','calctl'
  ]) AS id, 'calendar_scheduling'::text AS category
  UNION ALL
  SELECT unnest(ARRAY[
    'clawd-coach','meal-planner','sleep-calculator','withings-health','dexcom','healthkit-sync','healthkit-sync-apple'
  ]) AS id, 'health_fitness'::text AS category
  UNION ALL
  SELECT unnest(ARRAY[
    'smtp-send','whatsapp-styling-guide','vocal-chat','autoresponder','react-email-skills','apple-mail','bluesky','reddit','bird','outlook','imap-email','google-workspace','slack'
  ]) AS id, 'communication'::text AS category
  UNION ALL
  SELECT unnest(ARRAY[
    'home-assistant','apple-media','samsung-smart-tv','chromecast','sonoscli','camsnap','roku'
  ]) AS id, 'smart_home_iot'::text AS category
  UNION ALL
  SELECT unnest(ARRAY[
    'spotify','spotify-player','spotify-web-api','spotify-history','apple-music','youtube-api','youtube-summarizer','tmdb','content-advisory','overseerr','radarr','sonarr','trakt','plex','ytmusic','pocket-casts','lastfm','gifhorse','video-transcript-downloader'
  ]) AS id, 'media_streaming'::text AS category
  UNION ALL
  SELECT unnest(ARRAY[
    'smart-expense-tracker','copilot-money','ynab','monarch-money','yahoo-finance','just-fucking-cancel','card-optimizer','tax-professional','watch-my-money','refund-radar','plaid','expense-tracker-pro','ibkr-trading','george','financial-market-analysis'
  ]) AS id, 'finance'::text AS category
  UNION ALL
  SELECT unnest(ARRAY[
    'todo','doing-tasks','todoist','things-mac','ticktick','plan-my-day','morning-manifesto','trello','omnifocus','asana','linear','jira','clickup-mcp'
  ]) AS id, 'productivity_tasks'::text AS category
  UNION ALL
  SELECT unnest(ARRAY[
    'asr','openai-tts','gemini-stt','sag','voice-wake-say'
  ]) AS id, 'speech_transcription'::text AS category
  UNION ALL
  SELECT unnest(ARRAY[
    'pdf-tools','contract-reviewer','resume-builder'
  ]) AS id, 'documents'::text AS category
  UNION ALL
  SELECT unnest(ARRAY[
    'tavily','exa','firecrawl-search','proactive-research','serpapi','last30days','perplexity','kagi-search','news-aggregator','literature-review','gemini-deep-research','brave-search'
  ]) AS id, 'search_research'::text AS category
  UNION ALL
  SELECT unnest(ARRAY[
    'apple-contacts','apple-photos','apple-notes','icloud-findmy','shortcuts-generator','apple-mail-search','get-focus-mode','mole-mac-cleanup','alter-actions'
  ]) AS id, 'apple_apps_services'::text AS category
  UNION ALL
  SELECT unnest(ARRAY[
    'fal-ai','pollinations','veo','krea-api','coloring-page','gamma','excalidraw-flowchart','figma','video-frames'
  ]) AS id, 'image_video_gen'::text AS category
  UNION ALL
  SELECT unnest(ARRAY[
    'focus-mode','pros-cons','relationship-skills','thinking-partner','de-ai-ify','journal-to-post','self-improvement'
  ]) AS id, 'personal_development'::text AS category
  UNION ALL
  SELECT unnest(ARRAY[
    'apple-notes-skill','notion','better-notion','bear-notes','obsidian','craft','gkeep','sports-ticker','second-brain','granola','reflect'
  ]) AS id, 'notes_pkm'::text AS category
), normalized AS (
  SELECT
    id,
    category,
    CASE
      WHEN id = ANY(ARRAY[
        'asr','openai-tts','gemini-stt','sag','voice-wake-say','whatsapp-styling-guide','vocal-chat','autoresponder'
      ]::text[]) THEN 'gateway'::text
      WHEN id = ANY(ARRAY[
        'doing-tasks','plan-my-day','daily-rhythm','morning-manifesto','personal-shopper','clawringhouse',
        'smart-expense-tracker','card-optimizer','refund-radar','contract-reviewer','meeting-autopilot',
        'proactive-research','focus-mode','thinking-partner','relationship-skills','self-improvement'
      ]::text[]) THEN 'brain'::text
      ELSE 'hands'::text
    END AS plane,
    CASE
      WHEN id = ANY(ARRAY['george','lastfm']::text[]) THEN 'LOW'::text
      ELSE 'HIGH'::text
    END AS impact,
    CASE
      WHEN id = ANY(ARRAY[
        'apple-remind-me','calctl','healthkit-sync','healthkit-sync-apple','apple-mail','apple-mail-search',
        'apple-media','apple-music','apple-contacts','apple-photos','apple-notes','apple-notes-skill',
        'shortcuts-generator','get-focus-mode','mole-mac-cleanup','alter-actions','spotify','things-mac',
        'omnifocus','voice-wake-say','bear-notes','obsidian','craft','gamma'
      ]::text[]) THEN 'local_mac'::text
      WHEN id = ANY(ARRAY['google-calendar','clickup-mcp']::text[]) THEN 'mcp'::text
      ELSE 'cloud'::text
    END AS deployment_mode,
    initcap(replace(id, '-', ' ')) AS description,
    format('Execute %s workflow', id) AS brevio_use_case
  FROM seed
)
INSERT INTO skills.registry (
  id,
  category,
  plane,
  impact,
  description,
  brevio_use_case,
  input_schema,
  output_schema,
  config,
  required_scopes,
  retry_policy,
  circuit_breaker_config,
  cost_per_invocation_cents,
  enabled,
  min_tier,
  deployment_mode
)
SELECT
  id,
  category,
  plane,
  impact,
  description,
  brevio_use_case,
  '{}'::jsonb,
  '{}'::jsonb,
  '{}'::jsonb,
  ARRAY[]::text[],
  '{"max_retries":3,"initial_interval_ms":500,"backoff_multiplier":2.0,"max_interval_ms":8000,"non_retryable_errors":["AUTH_REVOKED","VALIDATION_FAILED"]}'::jsonb,
  '{"failure_threshold":5,"recovery_timeout_ms":60000,"half_open_max_calls":3}'::jsonb,
  0.0000,
  true,
  'free',
  deployment_mode
FROM normalized
ON CONFLICT (id) DO UPDATE
SET
  category = EXCLUDED.category,
  plane = EXCLUDED.plane,
  impact = EXCLUDED.impact,
  deployment_mode = EXCLUDED.deployment_mode,
  description = EXCLUDED.description,
  brevio_use_case = EXCLUDED.brevio_use_case,
  updated_at = now();

UPDATE skills.registry
SET plane = 'brain'
WHERE id = ANY(ARRAY[
  'personal-shopper','clawringhouse','daily-rhythm','meeting-autopilot','smart-expense-tracker','card-optimizer','refund-radar','doing-tasks','plan-my-day','morning-manifesto','proactive-research','focus-mode','relationship-skills','thinking-partner','self-improvement','contract-reviewer'
]);

UPDATE skills.registry
SET plane = 'gateway'
WHERE id = ANY(ARRAY[
  'asr','openai-tts','gemini-stt','sag','voice-wake-say','whatsapp-styling-guide','vocal-chat','autoresponder'
]);

UPDATE skills.registry
SET deployment_mode = 'local_mac'
WHERE id = ANY(ARRAY[
  'apple-remind-me','calctl','healthkit-sync','healthkit-sync-apple','apple-mail','apple-mail-search','apple-media','apple-music','apple-contacts','apple-photos','apple-notes','apple-notes-skill','shortcuts-generator','get-focus-mode','mole-mac-cleanup','alter-actions','spotify','things-mac','omnifocus','voice-wake-say','bear-notes','obsidian','craft','gamma'
]);

UPDATE skills.registry
SET deployment_mode = 'mcp'
WHERE id = ANY(ARRAY['google-calendar','clickup-mcp']);


WITH skill_metadata(id, description, brevio_use_case) AS (
VALUES
  ('aerobase-skill', 'Hands-plane adapter for flight search and itinerary comparison', 'Use aerobase-skill for hands-plane adapter for flight search and itinerary comparison.'),
  ('alter-actions', 'Hands-plane local app x-callback action orchestrator', 'Use alter-actions for hands-plane local app x-callback action orchestrator.'),
  ('apple-contacts', 'Local Apple Contacts search adapter', '"What''s Sarah''s phone number?" -> resolves contact details from local Apple Contacts'),
  ('apple-mail', 'Apple Mail adapter for inbox read/search and confirmation-gated send/reply actions', 'Use apple-mail for apple mail adapter for inbox read/search and confirmation-gated send/reply actions.'),
  ('apple-mail-search', 'Hands-plane low-latency Apple Mail indexed search adapter', 'Use apple-mail-search for hands-plane low-latency apple mail indexed search adapter.'),
  ('apple-media', 'Hands-plane local Apple media control adapter', 'Use apple-media for hands-plane local apple media control adapter.'),
  ('apple-music', 'Apple Music adapter for search, playback, and playlist update actions', 'Use apple-music for apple music adapter for search, playback, and playlist update actions.'),
  ('apple-notes', 'Hands-plane deprecated alias adapter for Apple Notes operations', 'Use apple-notes for hands-plane deprecated alias adapter for apple notes operations.'),
  ('apple-notes-skill', 'Notes/PKM adapter for apple-notes-skill operations', 'Save notes and search prior knowledge entries with deterministic routing'),
  ('apple-photos', 'Hands-plane Apple Photos query adapter', 'Use apple-photos for hands-plane apple photos query adapter.'),
  ('apple-remind-me', 'Hands-plane local Apple Reminders adapter for create/list/complete/delete flows', 'Use apple-remind-me for hands-plane local apple reminders adapter for create/list/complete/delete flows.'),
  ('asana', 'Asana task management adapter', '"Create an Asana task for this follow-up" with structured project routing'),
  ('asr', 'Gateway-plane speech-to-text skill for inbound voice message normalization', 'Use asr for gateway-plane speech-to-text skill for inbound voice message normalization.'),
  ('autoresponder', 'Gateway-plane hybrid interception skill with Brain delegation metadata', 'Use autoresponder for gateway-plane hybrid interception skill with brain delegation metadata.'),
  ('aviationstack-flight-tracker', 'Premium flight status tracking adapter with gate, terminal, and delay metadata', 'Use aviationstack-flight-tracker for premium flight status tracking adapter with gate, terminal, and delay metadata.'),
  ('bear-notes', 'Notes/PKM adapter for bear-notes operations', 'Save notes and search prior knowledge entries with deterministic routing'),
  ('better-notion', 'Hands-plane adapter for Notion page/database CRUD paths', 'Use better-notion for hands-plane adapter for notion page/database crud paths.'),
  ('bird', 'X/Twitter-style adapter for timeline/search and confirmation-gated posting', 'Use bird for x/twitter-style adapter for timeline/search and confirmation-gated posting.'),
  ('bluesky', 'Bluesky adapter for timeline retrieval, search, and confirmation-gated posting', 'Use bluesky for bluesky adapter for timeline retrieval, search, and confirmation-gated posting.'),
  ('brave-search', 'Privacy-oriented web search adapter', '"Search this topic with privacy-first results" for alternate provider routing'),
  ('buy-anything', 'Hands-plane shopping execution adapter for Amazon/Rye style checkout flows', 'Use buy-anything for hands-plane shopping execution adapter for amazon/rye style checkout flows.'),
  ('calctl', 'Hands-plane local Apple Calendar adapter for event create/list/update/cancel actions', 'Use calctl for hands-plane local apple calendar adapter for event create/list/update/cancel actions.'),
  ('camsnap', 'Hands-plane adapter for camera frame/clip capture workflows', 'Use camsnap for hands-plane adapter for camera frame/clip capture workflows.'),
  ('card-optimizer', 'Brain-plane card rewards recommendation adapter', 'Use card-optimizer for brain-plane card rewards recommendation adapter.'),
  ('chromecast', 'Hands-plane Chromecast control adapter for discovery and media playback', 'Use chromecast for hands-plane chromecast control adapter for discovery and media playback.'),
  ('clawd-coach', 'Hands-plane adapter for training-plan generation and session logging', 'Use clawd-coach for hands-plane adapter for training-plan generation and session logging.'),
  ('clawringhouse', 'Brain-plane proactive household shopping advisor', 'Use clawringhouse for brain-plane proactive household shopping advisor.'),
  ('clickup-mcp', 'ClickUp MCP-backed task/doc/time adapter', '"Create a ClickUp task" and "Start time tracking" through MCP routing'),
  ('coloring-page', 'Hands-plane adapter for converting prompts/images into printable coloring pages', 'Use coloring-page for hands-plane adapter for converting prompts/images into printable coloring pages.'),
  ('content-advisory', 'Hands-plane adapter for age/content advisory breakdowns', 'Use content-advisory for hands-plane adapter for age/content advisory breakdowns.'),
  ('contract-reviewer', 'Hands-plane adapter for contract risk triage and clause review hints', 'Use contract-reviewer for hands-plane adapter for contract risk triage and clause review hints.'),
  ('copilot-money', 'Copilot Money adapter for accounts, transactions, and net-worth summaries', 'Use copilot-money for copilot money adapter for accounts, transactions, and net-worth summaries.'),
  ('craft', 'Hands-plane adapter for Craft document create/append/search flows', 'Use craft for hands-plane adapter for craft document create/append/search flows.'),
  ('daily-rhythm', 'Brain-plane planning skill for structured daily briefings and wind-down prompts', 'Use daily-rhythm for brain-plane planning skill for structured daily briefings and wind-down prompts.'),
  ('de-ai-ify', 'Hands-plane adapter for rewriting AI-sounding text into a human voice', 'Use de-ai-ify for hands-plane adapter for rewriting ai-sounding text into a human voice.'),
  ('dexcom', 'Hands-plane glucose monitoring adapter for Dexcom CGM integrations', 'Use dexcom for hands-plane glucose monitoring adapter for dexcom cgm integrations.'),
  ('doing-tasks', 'Hands-plane orchestration adapter for routing tasks to downstream skills', 'Use doing-tasks for hands-plane orchestration adapter for routing tasks to downstream skills.'),
  ('exa', 'Neural web-search adapter for high-relevance research queries', '"Research executive planning frameworks" -> semantic results prioritized by relevance'),
  ('excalidraw-flowchart', 'Hands-plane adapter for generating and exporting Excalidraw flowcharts', 'Use excalidraw-flowchart for hands-plane adapter for generating and exporting excalidraw flowcharts.'),
  ('expense-tracker-pro', 'Hands-plane detailed expense tracker adapter', 'Use expense-tracker-pro for hands-plane detailed expense tracker adapter.'),
  ('fal-ai', 'Image generation adapter for Fal.ai model execution', '"Generate a social post hero image for tomorrow''s launch" -> validated image generation request'),
  ('figma', 'Hands-plane adapter for Figma analysis, export, and accessibility auditing', 'Use figma for hands-plane adapter for figma analysis, export, and accessibility auditing.'),
  ('financial-market-analysis', 'Market analysis adapter for sentiment, volatility, and correlation summaries', 'Use financial-market-analysis for market analysis adapter for sentiment, volatility, and correlation summaries.'),
  ('firecrawl-search', 'Web search + crawl adapter for richer content extraction', '"Find and summarize reviews" with crawl-ready content for downstream synthesis'),
  ('flight-tracker', 'Tracks flight status and live movement summaries using OpenSky-compatible responses', 'Use flight-tracker for tracks flight status and live movement summaries using opensky-compatible responses.'),
  ('focus-mode', 'Brain-plane focus execution skill for session lifecycle management', 'Use focus-mode for brain-plane focus execution skill for session lifecycle management.'),
  ('gamma', 'Hands-plane adapter for generating/updating/exporting Gamma decks', 'Use gamma for hands-plane adapter for generating/updating/exporting gamma decks.'),
  ('gemini-deep-research', 'Hands-plane adapter for deep research briefs', 'Use gemini-deep-research for hands-plane adapter for deep research briefs.'),
  ('gemini-stt', 'Gateway-plane premium STT skill with optional speaker labels', 'Use gemini-stt for gateway-plane premium stt skill with optional speaker labels.'),
  ('george', 'Hands-plane adapter for George banking account snapshots and analysis', 'Use george for hands-plane adapter for george banking account snapshots and analysis.'),
  ('get-focus-mode', 'Hands-plane adapter for reading macOS Focus mode context', 'Use get-focus-mode for hands-plane adapter for reading macos focus mode context.'),
  ('gifhorse', 'Hands-plane adapter for reaction GIF lookup', 'Use gifhorse for hands-plane adapter for reaction gif lookup.'),
  ('gkeep', 'Notes/PKM adapter for gkeep operations', 'Save notes and search prior knowledge entries with deterministic routing'),
  ('google-calendar', 'Calendar CRUD skill adapter', '"Schedule dinner with Sarah Friday at 7" -> creates event after confirmation'),
  ('google-maps', 'Route and travel estimation skill adapter', '"How long to the airport?" -> deterministic route estimate and steps'),
  ('google-workspace', 'Google Workspace unified mail/calendar/drive adapter', '"Send this to my team" and "What''s on my Google calendar today?" with one routed skill'),
  ('goplaces', 'Google Places style search adapter with optional location and open-now filters', 'Use goplaces for google places style search adapter with optional location and open-now filters.'),
  ('granola', 'Hands-plane adapter for meeting-note summarization and action extraction', 'Use granola for hands-plane adapter for meeting-note summarization and action extraction.'),
  ('grocery-list', 'Hands-plane grocery planning adapter for list CRUD and section-based organization', 'Use grocery-list for hands-plane grocery planning adapter for list crud and section-based organization.'),
  ('healthkit-sync', 'Hands-plane deprecated alias adapter for HealthKit sync', 'Use healthkit-sync for hands-plane deprecated alias adapter for healthkit sync.'),
  ('healthkit-sync-apple', 'Hands-plane canonical Apple HealthKit sync adapter', 'Use healthkit-sync-apple for hands-plane canonical apple healthkit sync adapter.'),
  ('home-assistant', 'Home automation control adapter', '"Turn off all the lights" or "Set thermostat to 72" with guardrails for sensitive actions'),
  ('ibkr-trading', 'Hands-plane adapter for IBKR quote/order/status workflows', 'Use ibkr-trading for hands-plane adapter for ibkr quote/order/status workflows.'),
  ('icloud-findmy', 'Find My device location adapter', '"Where are my AirPods?" -> returns latest available location and battery status'),
  ('imap-email', 'IMAP email adapter for generic mailbox listing/search plus confirmation-gated send', 'Use imap-email for imap email adapter for generic mailbox listing/search plus confirmation-gated send.'),
  ('jira', 'Jira issue management adapter', '"Open a Jira ticket for this incident" and transition workflow states'),
  ('journal-to-post', 'Hands-plane adapter for converting journal entries into social drafts', 'Use journal-to-post for hands-plane adapter for converting journal entries into social drafts.'),
  ('just-fucking-cancel', 'Hands-plane adapter for recurring subscription detection and cancellation draft prep', 'Use just-fucking-cancel for hands-plane adapter for recurring subscription detection and cancellation draft prep.'),
  ('kagi-search', 'Hands-plane adapter for Kagi-style web search', 'Use kagi-search for hands-plane adapter for kagi-style web search.'),
  ('krea-api', 'Hands-plane adapter for Krea image generation and upscaling', 'Use krea-api for hands-plane adapter for krea image generation and upscaling.'),
  ('last30days', 'Hands-plane trend scanner for recent cross-platform discussions', 'Use last30days for hands-plane trend scanner for recent cross-platform discussions.'),
  ('lastfm', 'Last.fm adapter for recent tracks, top tracks, and artist summary insights', 'Use lastfm for last.fm adapter for recent tracks, top tracks, and artist summary insights.'),
  ('linear', 'Linear issue management adapter', '"Create a Linear issue for this bug" or "Show open issues for ENG"'),
  ('literature-review', 'Hands-plane adapter for academic paper discovery and summarization prep', 'Use literature-review for hands-plane adapter for academic paper discovery and summarization prep.'),
  ('local-places', 'Nearby place search adapter optimized for quick local lookups', 'Use local-places for nearby place search adapter optimized for quick local lookups.'),
  ('marketplace', 'Hands-plane marketplace advisor for listing evaluation, price comparison, and listing draft copy', 'Use marketplace for hands-plane marketplace advisor for listing evaluation, price comparison, and listing draft copy.'),
  ('meal-planner', 'Hands-plane weekly meal planning and grocery rollup adapter', 'Use meal-planner for hands-plane weekly meal planning and grocery rollup adapter.'),
  ('meeting-autopilot', 'Brain-plane meeting synthesis skill for summaries, decisions, actions, and follow-up drafts', 'Use meeting-autopilot for brain-plane meeting synthesis skill for summaries, decisions, actions, and follow-up drafts.'),
  ('mole-mac-cleanup', 'Hands-plane adapter for macOS cleanup scanning and confirmed cleanup runs', 'Use mole-mac-cleanup for hands-plane adapter for macos cleanup scanning and confirmed cleanup runs.'),
  ('monarch-money', 'Monarch Money adapter for account, transaction, and budget summaries', 'Use monarch-money for monarch money adapter for account, transaction, and budget summaries.'),
  ('morning-manifesto', 'Brain-plane reflection and action framing skill for daily executive readiness', 'Use morning-manifesto for brain-plane reflection and action framing skill for daily executive readiness.'),
  ('news-aggregator', 'Multi-source news briefing adapter', '"What happened in tech today?" -> concise cross-source briefing list'),
  ('notion', 'Notion page/search mutation adapter', '"Add this to my project notes in Notion" -> create/append action with validated fields'),
  ('obsidian', 'Notes/PKM adapter for obsidian operations', 'Save notes and search prior knowledge entries with deterministic routing'),
  ('omnifocus', 'Hands-plane local OmniFocus adapter for GTD task operations', 'Use omnifocus for hands-plane local omnifocus adapter for gtd task operations.'),
  ('openai-tts', 'Gateway-plane text-to-speech skill using the OpenAI voice profile contract', 'Use openai-tts for gateway-plane text-to-speech skill using the openai voice profile contract.'),
  ('outlook', 'Microsoft Outlook mail/calendar adapter', '"Reply from Outlook" and "Show my Outlook calendar" with unified action routing'),
  ('overseerr', 'Hands-plane Overseerr adapter for searching and requesting media', 'Use overseerr for hands-plane overseerr adapter for searching and requesting media.'),
  ('parcel-package-tracking', 'Tracks global shipments with carrier inference and event timeline output', 'Use parcel-package-tracking for tracks global shipments with carrier inference and event timeline output.'),
  ('pdf-tools', 'Document utility adapter for text extraction, merge, and split operations', 'Use pdf-tools for document utility adapter for text extraction, merge, and split operations.'),
  ('perplexity', 'Web-grounded answer generation with citations', '"Give me a cited summary of recent developments" for research-grade responses'),
  ('personal-shopper', 'Brain-plane product research and ranking adapter for executive buying decisions', 'Use personal-shopper for brain-plane product research and ranking adapter for executive buying decisions.'),
  ('plaid', 'Plaid account/transaction/balance adapter', '"Show my latest transactions" and "What is my account balance?" with structured finance outputs'),
  ('plan-my-day', 'Brain-plane planning skill for deterministic task time-blocking and plan rebalancing', 'Use plan-my-day for brain-plane planning skill for deterministic task time-blocking and plan rebalancing.'),
  ('plex', 'Plex adapter for media search, recent listings, and playback control', 'Use plex for plex adapter for media search, recent listings, and playback control.'),
  ('pocket-casts', 'Pocket Casts adapter for queue listing, YouTube ingestion, and episode removal', 'Use pocket-casts for pocket casts adapter for queue listing, youtube ingestion, and episode removal.'),
  ('pollinations', 'Hands-plane adapter for Pollinations image/video/audio generation', 'Use pollinations for hands-plane adapter for pollinations image/video/audio generation.'),
  ('post-at', 'Hands-plane adapter for Austrian Post parcel tracking', 'Use post-at for hands-plane adapter for austrian post parcel tracking.'),
  ('proactive-research', 'Hands-plane adapter for topic monitoring and proactive update summaries', 'Use proactive-research for hands-plane adapter for topic monitoring and proactive update summaries.'),
  ('pros-cons', 'Hands-plane decision framework adapter for structured option scoring', 'Use pros-cons for hands-plane decision framework adapter for structured option scoring.'),
  ('radarr', 'Hands-plane Radarr adapter for movie search, add, and queue introspection', 'Use radarr for hands-plane radarr adapter for movie search, add, and queue introspection.'),
  ('react-email-skills', 'Hands-plane adapter for rendering React Email templates', 'Use react-email-skills for hands-plane adapter for rendering react email templates.'),
  ('recipe-to-list', 'Hands-plane adapter that transforms recipes into structured grocery tasks', 'Use recipe-to-list for hands-plane adapter that transforms recipes into structured grocery tasks.'),
  ('reddit', 'Reddit adapter for search, hot-post listing, and confirmation-gated posting', 'Use reddit for reddit adapter for search, hot-post listing, and confirmation-gated posting.'),
  ('reflect', 'Notes/PKM adapter for reflect operations', 'Save notes and search prior knowledge entries with deterministic routing'),
  ('refund-radar', 'Brain-plane recurring-charge scanner and refund-draft assistant', 'Use refund-radar for brain-plane recurring-charge scanner and refund-draft assistant.'),
  ('relationship-skills', 'Hands-plane coaching adapter for interpersonal communication planning', 'Use relationship-skills for hands-plane coaching adapter for interpersonal communication planning.'),
  ('resume-builder', 'Resume builder adapter for generation, tailoring, and scoring workflows', 'Use resume-builder for resume builder adapter for generation, tailoring, and scoring workflows.'),
  ('roku', 'Hands-plane adapter for Roku device control and status checks', 'Use roku for hands-plane adapter for roku device control and status checks.'),
  ('sag', 'Gateway-plane premium text-to-speech adapter (ElevenLabs-style output contract)', 'Use sag for gateway-plane premium text-to-speech adapter (elevenlabs-style output contract).'),
  ('samsung-smart-tv', 'Hands-plane SmartThings-backed Samsung TV control adapter', 'Use samsung-smart-tv for hands-plane smartthings-backed samsung tv control adapter.'),
  ('second-brain', 'Notes/PKM adapter for second-brain operations', 'Save notes and search prior knowledge entries with deterministic routing'),
  ('self-improvement', 'Hands-plane adapter for personal growth logging and weekly reflection', 'Use self-improvement for hands-plane adapter for personal growth logging and weekly reflection.'),
  ('serpapi', 'Unified multi-engine search adapter', '"Search products and reviews across platforms" with one connector'),
  ('shopping-expert', 'Shopping skill adapter for ranked product discovery', '"Find me running shoes under $100" -> ranked options with links and scores'),
  ('shortcuts-generator', 'Hands-plane local Apple Shortcuts generation/install adapter', 'Use shortcuts-generator for hands-plane local apple shortcuts generation/install adapter.'),
  ('slack', 'Slack collaboration adapter for channel listing, posting, and reactions', 'Use slack for slack collaboration adapter for channel listing, posting, and reactions.'),
  ('sleep-calculator', 'Hands-plane sleep-cycle planning adapter', 'Use sleep-calculator for hands-plane sleep-cycle planning adapter.'),
  ('smart-expense-tracker', 'Brain-plane expense orchestration adapter for logging, budget status, and daily briefings', 'Use smart-expense-tracker for brain-plane expense orchestration adapter for logging, budget status, and daily briefings.'),
  ('smtp-send', 'Email send adapter over SMTP semantics', '"Email my landlord about the dishwasher" -> draft + confirmation + send'),
  ('sonarr', 'Hands-plane Sonarr adapter for series search, add, and queue introspection', 'Use sonarr for hands-plane sonarr adapter for series search, add, and queue introspection.'),
  ('sonoscli', 'Hands-plane Sonos multi-speaker control adapter', 'Use sonoscli for hands-plane sonos multi-speaker control adapter.'),
  ('sports-ticker', 'Hands-plane adapter for live scores and schedule lookups', 'Use sports-ticker for hands-plane adapter for live scores and schedule lookups.'),
  ('spotify', 'Hands-plane local Spotify playback control adapter', 'Use spotify for hands-plane local spotify playback control adapter.'),
  ('spotify-history', 'Hands-plane Spotify listening analytics adapter', 'Use spotify-history for hands-plane spotify listening analytics adapter.'),
  ('spotify-player', 'Hands-plane terminal-style Spotify queue/search control adapter', 'Use spotify-player for hands-plane terminal-style spotify queue/search control adapter.'),
  ('spotify-web-api', 'Spotify playback/search/history adapter', '"Play my focus playlist" or "What have I listened to this month?" routed by disambiguation rules'),
  ('spots', 'Exhaustive area scanning adapter for dense places discovery', 'Use spots for exhaustive area scanning adapter for dense places discovery.'),
  ('swissweather', 'Hands-plane adapter for Swiss weather forecasting', 'Use swissweather for hands-plane adapter for swiss weather forecasting.'),
  ('tavily', 'Web search adapter for concise research retrieval', 'Research question -> ranked web-grounded sources for Brain aggregation'),
  ('tax-professional', 'Hands-plane tax planning assistant adapter', 'Use tax-professional for hands-plane tax planning assistant adapter.'),
  ('things-mac', 'Hands-plane local Things 3 adapter for task lifecycle operations', 'Use things-mac for hands-plane local things 3 adapter for task lifecycle operations.'),
  ('thinking-partner', 'Brain-plane reasoning skill for clarifying problems, testing assumptions, and structuring choices', 'Use thinking-partner for brain-plane reasoning skill for clarifying problems, testing assumptions, and structuring choices.'),
  ('ticktick', 'Hands-plane TickTick task adapter with typed task CRUD-like actions', 'Use ticktick for hands-plane ticktick task adapter with typed task crud-like actions.'),
  ('tmdb', 'TMDB recommendation/search adapter', '"What should I watch tonight?" -> ranked recommendations with streaming availability'),
  ('todo', 'Generic task list adapter for local task operations', '"Add this to my tasks" and quick task mutations when no provider preference is set'),
  ('todoist', 'Task management adapter for Todoist action workflows', '"Add finish board memo to my work tasks" -> creates structured Todoist task metadata'),
  ('track17', 'Multi-carrier package tracking adapter for 17TRACK style event feeds', 'Use track17 for multi-carrier package tracking adapter for 17track style event feeds.'),
  ('trakt', 'Trakt adapter for watch history, trending items, and watched-state updates', 'Use trakt for trakt adapter for watch history, trending items, and watched-state updates.'),
  ('trello', 'Trello card management adapter', '"Move this card to Done" and board task automation flows'),
  ('veo', 'Hands-plane adapter for Veo video generation and job status checks', 'Use veo for hands-plane adapter for veo video generation and job status checks.'),
  ('video-frames', 'Hands-plane video frame extraction adapter', 'Use video-frames for hands-plane video frame extraction adapter.'),
  ('video-transcript-downloader', 'Hands-plane transcript/subtitle extraction adapter for online video content', 'Use video-transcript-downloader for hands-plane transcript/subtitle extraction adapter for online video content.'),
  ('vocal-chat', 'Gateway-plane end-to-end voice pipeline skill (STT + response + TTS contract)', 'Use vocal-chat for gateway-plane end-to-end voice pipeline skill (stt + response + tts contract).'),
  ('voice-wake-say', 'Gateway-plane local fallback TTS skill using macOS `say` command semantics', 'Use voice-wake-say for gateway-plane local fallback tts skill using macos `say` command semantics.'),
  ('watch-my-money', 'Hands-plane statement analysis and budget alert adapter', 'Use watch-my-money for hands-plane statement analysis and budget alert adapter.'),
  ('whatsapp-styling-guide', 'Gateway-plane channel formatter for WhatsApp-specific response presentation', 'Use whatsapp-styling-guide for gateway-plane channel formatter for whatsapp-specific response presentation.'),
  ('withings-health', 'Hands-plane health metrics adapter for Withings device measurements', 'Use withings-health for hands-plane health metrics adapter for withings device measurements.'),
  ('yahoo-finance', 'Yahoo Finance adapter for quotes, fundamentals, and market news', 'Use yahoo-finance for yahoo finance adapter for quotes, fundamentals, and market news.'),
  ('ynab', 'Budget data adapter for YNAB account and transaction workflows', '"How much is left in my budget this month?" -> summary/accounts/transactions context for response generation'),
  ('youtube-api', 'YouTube search/transcript/channel adapter', '"Summarize this YouTube video" -> transcript summary routed to the response generator'),
  ('youtube-summarizer', 'Hands-plane YouTube transcript summarization adapter', 'Use youtube-summarizer for hands-plane youtube transcript summarization adapter.'),
  ('ytmusic', 'YouTube Music adapter for search, playback, and queueing actions', 'Use ytmusic for youtube music adapter for search, playback, and queueing actions.')
)
UPDATE skills.registry AS registry
SET
  description = skill_metadata.description,
  brevio_use_case = skill_metadata.brevio_use_case,
  updated_at = now()
FROM skill_metadata
WHERE registry.id = skill_metadata.id;

COMMIT;

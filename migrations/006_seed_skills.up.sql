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

COMMIT;

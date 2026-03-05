BEGIN;

DELETE FROM skills.registry
WHERE id = ANY(ARRAY[
  'shopping-expert','buy-anything','personal-shopper','grocery-list','marketplace','clawringhouse','recipe-to-list',
  'google-maps','flight-tracker','aviationstack-flight-tracker','aerobase-skill','parcel-package-tracking','track17','post-at','spots','local-places','goplaces','swissweather',
  'google-calendar','apple-remind-me','daily-rhythm','meeting-autopilot','calctl',
  'clawd-coach','meal-planner','sleep-calculator','withings-health','dexcom','healthkit-sync','healthkit-sync-apple',
  'smtp-send','whatsapp-styling-guide','vocal-chat','autoresponder','react-email-skills','apple-mail','bluesky','reddit','bird','outlook','imap-email','google-workspace','slack',
  'home-assistant','apple-media','samsung-smart-tv','chromecast','sonoscli','camsnap','roku',
  'spotify','spotify-player','spotify-web-api','spotify-history','apple-music','youtube-api','youtube-summarizer','tmdb','content-advisory','overseerr','radarr','sonarr','trakt','plex','ytmusic','pocket-casts','lastfm','gifhorse','video-transcript-downloader',
  'smart-expense-tracker','copilot-money','ynab','monarch-money','yahoo-finance','just-fucking-cancel','card-optimizer','tax-professional','watch-my-money','refund-radar','plaid','expense-tracker-pro','ibkr-trading','george','financial-market-analysis',
  'todo','doing-tasks','todoist','things-mac','ticktick','plan-my-day','morning-manifesto','trello','omnifocus','asana','linear','jira','clickup-mcp',
  'asr','openai-tts','gemini-stt','sag','voice-wake-say',
  'pdf-tools','contract-reviewer','resume-builder',
  'tavily','exa','firecrawl-search','proactive-research','serpapi','last30days','perplexity','kagi-search','news-aggregator','literature-review','gemini-deep-research','brave-search',
  'apple-contacts','apple-photos','apple-notes','icloud-findmy','shortcuts-generator','apple-mail-search','get-focus-mode','mole-mac-cleanup','alter-actions',
  'fal-ai','pollinations','veo','krea-api','coloring-page','gamma','excalidraw-flowchart','figma','video-frames',
  'focus-mode','pros-cons','relationship-skills','thinking-partner','de-ai-ify','journal-to-post','self-improvement',
  'apple-notes-skill','notion','better-notion','bear-notes','obsidian','craft','gkeep','sports-ticker','second-brain','granola','reflect'
]);

COMMIT;

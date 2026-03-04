package contracts

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestHandsPrioritySkillsNoLongerScaffolded(t *testing.T) {
	t.Parallel()

	root := repositoryRoot(t)
	skillsRoot := filepath.Join(root, "services", "brevio-hands", "src", "skills")
	scriptPath := filepath.Join(root, "scripts", "skills", "generate_hands_skill_scaffolds.sh")
	manualOverridePath := filepath.Join(root, "config", "skill-manual-overrides.txt")

	schemaTokens := map[string][]string{
		"shopping-expert": {
			"query",
			"results",
			"mock_catalog",
		},
		"google-maps": {
			"origin",
			"destination",
			"distance_m",
		},
		"google-calendar": {
			"action",
			"confirmation_required",
		},
		"tavily": {
			"query",
			"results",
			"provider",
			"tavily",
		},
		"smtp-send": {
			"to",
			"subject",
			"confirmation_required",
			"confirmed",
		},
		"home-assistant": {
			"entity_id",
			"two_factor_code",
		},
		"todoist": {
			"action",
			"project_id",
			"task",
		},
		"youtube-api": {
			"mode",
			"video_id",
			"transcript",
		},
		"ynab": {
			"action",
			"budget_id",
			"total_budget_cents",
		},
		"notion": {
			"action",
			"page_id",
			"create_page",
		},
		"fal-ai": {
			"prompt",
			"image_url",
			"model_used",
		},
		"apple-contacts": {
			"query",
			"contacts",
			"apple-contacts-local",
		},
		"spotify-web-api": {
			"action",
			"top_tracks",
			"playing",
		},
		"tmdb": {
			"query",
			"results",
			"streaming",
		},
		"plaid": {
			"action",
			"account_id",
			"balances",
		},
		"google-workspace": {
			"action",
			"gmail_send",
			"confirmation_required",
		},
		"outlook": {
			"action",
			"send",
			"confirmation_required",
		},
		"icloud-findmy": {
			"device_name",
			"devices",
			"icloud-findmy",
		},
		"exa": {
			"query",
			"results",
			"score",
		},
		"serpapi": {
			"query",
			"engine",
			"results",
		},
		"perplexity": {
			"query",
			"answer",
			"citations",
		},
		"brave-search": {
			"query",
			"results",
			"brave-search",
		},
		"firecrawl-search": {
			"query",
			"content",
			"firecrawl",
		},
		"news-aggregator": {
			"topic",
			"items",
			"news-aggregator",
		},
		"linear": {
			"action",
			"issue_create",
			"issues",
		},
		"jira": {
			"action",
			"issue_transition",
			"issue_key",
		},
		"asana": {
			"action",
			"task_create",
			"tasks",
		},
		"trello": {
			"action",
			"card_move",
			"cards",
		},
		"clickup-mcp": {
			"action",
			"doc_create",
			"timer_started",
		},
		"todo": {
			"action",
			"complete",
			"items",
		},
		"apple-notes-skill": {
			"action",
			"search",
			"notes",
		},
		"gkeep": {
			"action",
			"create",
			"notes",
		},
		"bear-notes": {
			"action",
			"update",
			"notes",
		},
		"obsidian": {
			"action",
			"search",
			"notes",
		},
		"reflect": {
			"action",
			"create",
			"notes",
		},
		"second-brain": {
			"action",
			"search",
			"notes",
		},
		"flight-tracker": {
			"callsign",
			"icao24",
			"origin_iata",
		},
		"aviationstack-flight-tracker": {
			"flight_iata",
			"status",
			"queried_at_utc",
		},
		"parcel-package-tracking": {
			"tracking_number",
			"carrier",
			"history",
		},
		"track17": {
			"tracking_number",
			"checkpoints",
			"17track",
		},
		"goplaces": {
			"query",
			"location",
			"results",
		},
		"local-places": {
			"query",
			"radius_km",
			"results",
		},
		"spots": {
			"query",
			"grid_density",
			"results",
		},
		"apple-mail": {
			"action",
			"to",
			"subject",
		},
		"imap-email": {
			"action",
			"mailbox",
			"messages",
		},
		"slack": {
			"action",
			"channel_id",
			"emoji",
		},
		"reddit": {
			"action",
			"subreddit",
			"posts",
		},
		"bluesky": {
			"action",
			"query",
			"posts",
		},
		"bird": {
			"action",
			"query",
			"posts",
		},
		"apple-music": {
			"action",
			"playlist_id",
			"tracks",
		},
		"ytmusic": {
			"action",
			"track_id",
			"tracks",
		},
		"plex": {
			"action",
			"media_id",
			"results",
		},
		"trakt": {
			"action",
			"media_id",
			"items",
		},
		"lastfm": {
			"action",
			"username",
			"tracks",
		},
		"pocket-casts": {
			"action",
			"youtube_url",
			"queue",
		},
		"copilot-money": {
			"action",
			"account_id",
			"net_worth_cents",
		},
		"monarch-money": {
			"action",
			"month",
			"budgets",
		},
		"yahoo-finance": {
			"action",
			"symbols",
			"disclaimer",
		},
		"financial-market-analysis": {
			"action",
			"symbols",
			"correlation",
		},
		"pdf-tools": {
			"action",
			"files",
			"page_range",
		},
		"resume-builder": {
			"action",
			"role",
			"recommendations",
		},
		"restaurant-reservations": {
			"action",
			"party_size",
			"partnership_status",
		},
		"food-delivery-ordering": {
			"action",
			"cart_id",
			"partnership_status",
		},
		"ride-hailing": {
			"action",
			"service_tier",
			"partnership_status",
		},
		"hotel-vacation-booking": {
			"action",
			"check_in",
			"partnership_status",
		},
		"bill-pay-p2p": {
			"action",
			"amount_cents",
			"partnership_status",
		},
		"streaming-recommendations": {
			"action",
			"watchlist_added",
			"partnership_status",
		},
		"local-service-booking": {
			"action",
			"provider_id",
			"partnership_status",
		},
		"kids-family-management": {
			"action",
			"child_name",
			"partnership_status",
		},
		"pharmacy-prescription": {
			"action",
			"prescription_id",
			"partnership_status",
		},
		"pet-care": {
			"action",
			"service_type",
			"partnership_status",
		},
		"daily-rhythm": {
			"action",
			"wake_time_local",
			"schedule_blocks",
		},
		"plan-my-day": {
			"action",
			"tasks",
			"time_blocks",
		},
		"morning-manifesto": {
			"action",
			"goals",
			"manifesto",
		},
		"meeting-autopilot": {
			"action",
			"transcript",
			"action_items",
		},
		"thinking-partner": {
			"action",
			"questions",
			"decision_matrix",
		},
		"focus-mode": {
			"action",
			"session_id",
			"check_in_schedule",
		},
		"asr": {
			"audio_url",
			"transcript",
			"latency_budget_ms",
		},
		"gemini-stt": {
			"audio_url",
			"speakers",
			"latency_budget_ms",
		},
		"openai-tts": {
			"text",
			"audio_url",
			"latency_budget_ms",
		},
		"sag": {
			"text",
			"voice_id",
			"latency_budget_ms",
		},
		"voice-wake-say": {
			"text",
			"command",
			"latency_budget_ms",
		},
		"whatsapp-styling-guide": {
			"text",
			"formatted_text",
			"applied_rules",
		},
		"vocal-chat": {
			"audio_url",
			"reply_audio_url",
			"latency_budget_ms",
		},
		"autoresponder": {
			"action",
			"delegated_to_brain",
			"latency_budget_ms",
		},
		"buy-anything": {
			"action",
			"line_items",
			"checkout_preview",
		},
		"grocery-list": {
			"action",
			"items",
			"total_items",
		},
		"recipe-to-list": {
			"action",
			"recipe_items",
			"normalized_items",
		},
		"marketplace": {
			"action",
			"fair_price_cents",
			"scam_risk",
		},
		"personal-shopper": {
			"action",
			"ranked_candidates",
			"recommendation",
		},
		"clawringhouse": {
			"action",
			"household_items",
			"recommendations",
		},
		"withings-health": {
			"measure_type",
			"measurements",
			"trend",
		},
		"dexcom": {
			"action",
			"readings",
			"alerts",
		},
		"healthkit-sync": {
			"alias_target",
			"deprecated_alias",
			"forwarded",
		},
		"healthkit-sync-apple": {
			"action",
			"snapshots",
			"synced_metric_count",
		},
		"sleep-calculator": {
			"action",
			"recommendations",
			"sleep_cycles",
		},
		"meal-planner": {
			"action",
			"meals",
			"grocery_items",
		},
		"apple-media": {
			"action",
			"device_name",
			"now_playing",
		},
		"apple-photos": {
			"action",
			"albums",
			"photos",
		},
		"apple-notes": {
			"canonical_skill_id",
			"deprecated_alias",
			"notes",
		},
		"apple-mail-search": {
			"query",
			"results",
			"latency_profile_ms",
		},
		"alter-actions": {
			"action",
			"actions",
			"callback_url",
		},
		"get-focus-mode": {
			"action",
			"current_mode",
			"schedule",
		},
	}

	indexTokens := map[string][]string{
		"shopping-expert": {"VALIDATION_FAILED"},
		"google-maps":     {"VALIDATION_FAILED"},
		"google-calendar": {"requiredScopes", "calendar"},
		"tavily":          {"VALIDATION_FAILED"},
		"smtp-send":       {"confirmed", "confirmation_required"},
		"home-assistant":  {"SAFETY_2FA_REQUIRED", "Action requires 2FA confirmation"},
		"todoist":         {"requiredScopes", "TODOIST_CONTENT_REQUIRED"},
		"youtube-api":     {"YOUTUBE_VIDEO_ID_REQUIRED"},
		"ynab":            {"requiredScopes", "YNAB_ACCOUNT_NOT_FOUND"},
		"notion":          {"requiredScopes", "NOTION_TITLE_REQUIRED"},
		"fal-ai":          {"FAL_CONTENT_POLICY_BLOCKED"},
		"apple-contacts":  {"apple-contacts-local"},
		"spotify-web-api": {"requiredScopes", "user-modify-playback-state"},
		"tmdb":            {"tmdb execution failed"},
		"plaid":           {"PLAID_ACCOUNT_NOT_FOUND"},
		"google-workspace": {
			"requiredScopes",
			"GOOGLE_WORKSPACE_SEND_FIELDS_REQUIRED",
		},
		"outlook":       {"requiredScopes", "OUTLOOK_SEND_FIELDS_REQUIRED"},
		"icloud-findmy": {"icloud-findmy execution failed"},
		"exa":           {"exa execution failed"},
		"serpapi":       {"serpapi execution failed"},
		"perplexity":    {"perplexity execution failed"},
		"brave-search":  {"brave-search execution failed"},
		"firecrawl-search": {
			"firecrawl-search execution failed",
		},
		"news-aggregator": {"news-aggregator execution failed"},
		"linear":          {"LINEAR_CREATE_FIELDS_REQUIRED"},
		"jira":            {"JIRA_CREATE_FIELDS_REQUIRED"},
		"asana":           {"ASANA_CREATE_FIELDS_REQUIRED"},
		"trello":          {"TRELLO_CREATE_FIELDS_REQUIRED"},
		"clickup-mcp":     {"CLICKUP_TITLE_REQUIRED"},
		"todo":            {"TODO_CONTENT_REQUIRED"},
		"apple-notes-skill": {
			"APPLE_NOTES_SKILL_CREATE_FIELDS_REQUIRED",
		},
		"gkeep": {"GKEEP_CREATE_FIELDS_REQUIRED"},
		"bear-notes": {
			"BEAR_NOTES_CREATE_FIELDS_REQUIRED",
		},
		"obsidian": {"OBSIDIAN_CREATE_FIELDS_REQUIRED"},
		"reflect":  {"REFLECT_CREATE_FIELDS_REQUIRED"},
		"second-brain": {
			"SECOND_BRAIN_CREATE_FIELDS_REQUIRED",
		},
		"flight-tracker": {"FLIGHT_TRACKER_IDENTIFIER_REQUIRED"},
		"aviationstack-flight-tracker": {
			"AVIATIONSTACK_FLIGHT_IDENTIFIER_REQUIRED",
		},
		"parcel-package-tracking": {
			"parcel-package-tracking execution failed",
		},
		"track17": {"track17 execution failed"},
		"goplaces": {
			"requiredScopes",
			"google.places.read",
		},
		"local-places": {"local-places execution failed"},
		"spots":        {"spots execution failed"},
		"apple-mail":   {"APPLE_MAIL_CONFIRMATION_REQUIRED"},
		"imap-email":   {"IMAP_EMAIL_CONFIRMATION_REQUIRED"},
		"slack": {
			"requiredScopes",
			"chat:write",
		},
		"reddit":  {"REDDIT_POST_CONFIRMATION_REQUIRED"},
		"bluesky": {"BLUESKY_POST_CONFIRMATION_REQUIRED"},
		"bird":    {"BIRD_POST_CONFIRMATION_REQUIRED"},
		"apple-music": {
			"requiredScopes",
			"apple.music.modify",
		},
		"ytmusic":      {"ytmusic execution failed"},
		"plex":         {"plex execution failed"},
		"trakt":        {"trakt execution failed"},
		"lastfm":       {"lastfm execution failed"},
		"pocket-casts": {"pocket-casts execution failed"},
		"copilot-money": {
			"COPILOT_MONEY_ACCOUNT_REQUIRED",
		},
		"monarch-money": {"MONARCH_MONEY_ACCOUNT_REQUIRED"},
		"yahoo-finance": {"YAHOO_FINANCE_SYMBOLS_REQUIRED"},
		"financial-market-analysis": {
			"FINANCIAL_MARKET_ANALYSIS_SYMBOLS_REQUIRED",
		},
		"pdf-tools":      {"PDF_TOOLS_MERGE_FILES_REQUIRED"},
		"resume-builder": {"RESUME_BUILDER_ROLE_REQUIRED"},
		"restaurant-reservations": {
			"RESTAURANT_RESERVATIONS_CONFIRMATION_REQUIRED",
			"CUSTOM_BUILD_REQUIRED",
		},
		"food-delivery-ordering": {
			"FOOD_DELIVERY_CHECKOUT_CONFIRMATION_REQUIRED",
			"CUSTOM_BUILD_REQUIRED",
		},
		"ride-hailing": {
			"RIDE_HAILING_CONFIRMATION_REQUIRED",
			"CUSTOM_BUILD_REQUIRED",
		},
		"hotel-vacation-booking": {
			"HOTEL_BOOKING_CONFIRMATION_REQUIRED",
			"CUSTOM_BUILD_REQUIRED",
		},
		"bill-pay-p2p": {
			"BILL_PAY_CONFIRMATION_REQUIRED",
			"CUSTOM_BUILD_REQUIRED",
		},
		"streaming-recommendations": {
			"STREAMING_RECOMMENDATIONS_CONFIRMATION_REQUIRED",
			"CUSTOM_BUILD_REQUIRED",
		},
		"local-service-booking": {
			"LOCAL_SERVICE_BOOKING_CONFIRMATION_REQUIRED",
			"CUSTOM_BUILD_REQUIRED",
		},
		"kids-family-management": {"CUSTOM_BUILD_REQUIRED"},
		"pharmacy-prescription": {
			"PHARMACY_REFILL_CONFIRMATION_REQUIRED",
			"CUSTOM_BUILD_REQUIRED",
		},
		"pet-care": {
			"PET_CARE_CONFIRMATION_REQUIRED",
			"CUSTOM_BUILD_REQUIRED",
		},
		"daily-rhythm": {
			"DAILY_RHYTHM_CONTEXT_REQUIRED",
		},
		"plan-my-day": {"PLAN_MY_DAY_TASKS_REQUIRED"},
		"morning-manifesto": {
			"MORNING_MANIFESTO_GOALS_REQUIRED",
		},
		"meeting-autopilot": {
			"MEETING_AUTOPILOT_TRANSCRIPT_REQUIRED",
		},
		"thinking-partner": {
			"THINKING_PARTNER_TOPIC_REQUIRED",
		},
		"focus-mode": {
			"FOCUS_MODE_SESSION_REQUIRED",
		},
		"asr": {"ASR_AUDIO_URL_REQUIRED"},
		"gemini-stt": {
			"GEMINI_STT_AUDIO_URL_REQUIRED",
		},
		"openai-tts": {"OPENAI_TTS_TEXT_REQUIRED"},
		"sag":        {"SAG_TEXT_REQUIRED"},
		"voice-wake-say": {
			"VOICE_WAKE_SAY_TEXT_REQUIRED",
		},
		"whatsapp-styling-guide": {
			"WHATSAPP_STYLING_TEXT_REQUIRED",
		},
		"vocal-chat": {"VOCAL_CHAT_AUDIO_REQUIRED"},
		"autoresponder": {
			"AUTORESPONDER_INTERCEPT_TEXT_REQUIRED",
		},
		"buy-anything": {"BUY_ANYTHING_ORDER_CONFIRMATION_REQUIRED"},
		"grocery-list": {
			"GROCERY_LIST_CLEAR_CONFIRMATION_REQUIRED",
		},
		"recipe-to-list": {"RECIPE_TO_LIST_TEXT_REQUIRED"},
		"marketplace":    {"MARKETPLACE_TITLE_REQUIRED"},
		"personal-shopper": {
			"PERSONAL_SHOPPER_QUERY_REQUIRED",
		},
		"clawringhouse": {"CLAWRINGHOUSE_ITEMS_REQUIRED"},
		"withings-health": {
			"WITHINGS_MEASURE_TYPE_REQUIRED",
		},
		"dexcom": {"DEXCOM_TIME_RANGE_REQUIRED"},
		"healthkit-sync": {
			"HEALTHKIT_SYNC_ALIAS_RANGE_REQUIRED",
		},
		"healthkit-sync-apple": {
			"HEALTHKIT_SYNC_APPLE_RANGE_REQUIRED",
		},
		"sleep-calculator": {
			"SLEEP_CALCULATOR_WAKE_TIME_REQUIRED",
		},
		"meal-planner": {"MEAL_PLANNER_HOUSEHOLD_SIZE_REQUIRED"},
		"apple-media": {
			"APPLE_MEDIA_DEVICE_REQUIRED",
		},
		"apple-photos": {"APPLE_PHOTOS_QUERY_REQUIRED"},
		"apple-notes":  {"APPLE_NOTES_CREATE_FIELDS_REQUIRED"},
		"apple-mail-search": {
			"APPLE_MAIL_SEARCH_QUERY_REQUIRED",
		},
		"alter-actions": {
			"ALTER_ACTIONS_CONFIRMATION_REQUIRED",
		},
		"get-focus-mode": {
			"focus_mode.read",
		},
	}

	scriptBody, err := os.ReadFile(scriptPath)
	if err != nil {
		t.Fatalf("read skill scaffold script: %v", err)
	}
	scriptText := string(scriptBody)
	if !strings.Contains(scriptText, "config/skill-manual-overrides.txt") {
		t.Fatalf("skill scaffold script missing manual override config reference")
	}

	overrideBody, err := os.ReadFile(manualOverridePath)
	if err != nil {
		t.Fatalf("read manual override file: %v", err)
	}
	overrideText := string(overrideBody)

	for skillID, tokens := range schemaTokens {
		skillDir := filepath.Join(skillsRoot, skillID)
		indexPath := filepath.Join(skillDir, "index.ts")
		schemaPath := filepath.Join(skillDir, "schema.ts")
		readmePath := filepath.Join(skillDir, "README.md")

		assertFileContainsTokens(t, indexPath, append([]string{skillID}, indexTokens[skillID]...))
		assertFileContainsTokens(t, schemaPath, tokens)

		readmeBody, readErr := os.ReadFile(readmePath)
		if readErr != nil {
			t.Fatalf("read %s readme: %v", skillID, readErr)
		}
		if strings.Contains(strings.ToLower(string(readmeBody)), "generated skill adapter scaffold") {
			t.Fatalf("priority skill %s README still contains scaffold marker", skillID)
		}

		if !strings.Contains(overrideText, skillID) {
			t.Fatalf("manual override list missing %s", skillID)
		}
	}
}

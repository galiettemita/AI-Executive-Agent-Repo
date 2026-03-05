package connectors

type APIKeyServiceConfig struct {
	ServiceKey  string
	KeySource   string
	SkillsUsing []string
	RateLimit   string
	CostModel   string
}

type NoAuthServiceConfig struct {
	ServiceKey  string
	SkillsUsing []string
	Notes       string
}

func APIKeyServiceRegistry() map[string]APIKeyServiceConfig {
	return map[string]APIKeyServiceConfig{
		"tavily":                 {ServiceKey: "tavily", KeySource: "tavily.com dashboard", SkillsUsing: []string{"tavily"}, RateLimit: "1000/day free, 10k/day paid", CostModel: "$0.01/search"},
		"exa":                    {ServiceKey: "exa", KeySource: "exa.ai dashboard", SkillsUsing: []string{"exa"}, RateLimit: "1000/month free", CostModel: "$0.001/search"},
		"firecrawl":              {ServiceKey: "firecrawl", KeySource: "firecrawl.dev dashboard", SkillsUsing: []string{"firecrawl-search"}, RateLimit: "500/month free", CostModel: "$0.004/page"},
		"serpapi":                {ServiceKey: "serpapi", KeySource: "serpapi.com dashboard", SkillsUsing: []string{"serpapi"}, RateLimit: "100/month free", CostModel: "$0.01/search"},
		"perplexity":             {ServiceKey: "perplexity", KeySource: "perplexity.ai/api dashboard", SkillsUsing: []string{"perplexity"}, RateLimit: "provider limited", CostModel: "$0.005/query"},
		"kagi":                   {ServiceKey: "kagi", KeySource: "kagi.com/settings/api", SkillsUsing: []string{"kagi-search"}, RateLimit: "50/day basic", CostModel: "$0.01/search"},
		"brave-search":           {ServiceKey: "brave-search", KeySource: "brave.com/search/api", SkillsUsing: []string{"brave-search"}, RateLimit: "2000/month free", CostModel: "$0.003/search"},
		"openai":                 {ServiceKey: "openai", KeySource: "platform.openai.com", SkillsUsing: []string{"openai-tts"}, RateLimit: "500 req/min", CostModel: "$0.015/1k chars (tts)"},
		"elevenlabs":             {ServiceKey: "elevenlabs", KeySource: "elevenlabs.io dashboard", SkillsUsing: []string{"sag"}, RateLimit: "10k chars/month free", CostModel: "$0.003/100 chars"},
		"fal-ai":                 {ServiceKey: "fal-ai", KeySource: "fal.ai dashboard", SkillsUsing: []string{"fal-ai"}, RateLimit: "pay per use", CostModel: "$0.01-$0.10/image"},
		"pollinations":           {ServiceKey: "pollinations", KeySource: "pollinations.ai", SkillsUsing: []string{"pollinations"}, RateLimit: "generous free tier", CostModel: "free"},
		"krea":                   {ServiceKey: "krea", KeySource: "krea.ai dashboard", SkillsUsing: []string{"krea-api"}, RateLimit: "pay per use", CostModel: "$0.02-$0.05/image"},
		"tmdb":                   {ServiceKey: "tmdb", KeySource: "themoviedb.org/settings/api", SkillsUsing: []string{"tmdb", "content-advisory"}, RateLimit: "40 req/10s", CostModel: "free"},
		"opensky":                {ServiceKey: "opensky", KeySource: "opensky-network.org", SkillsUsing: []string{"flight-tracker"}, RateLimit: "100/day anon, 4000/day auth", CostModel: "free"},
		"aviationstack":          {ServiceKey: "aviationstack", KeySource: "aviationstack.com dashboard", SkillsUsing: []string{"aviationstack-flight-tracker"}, RateLimit: "500/month free", CostModel: "$0.01/request"},
		"17track":                {ServiceKey: "17track", KeySource: "api.17track.net dashboard", SkillsUsing: []string{"track17"}, RateLimit: "100/day free", CostModel: "$0.01/tracking"},
		"parcel":                 {ServiceKey: "parcel", KeySource: "parcel-api.com", SkillsUsing: []string{"parcel-package-tracking"}, RateLimit: "provider limited", CostModel: "$0.005/track"},
		"home-assistant-api-key": {ServiceKey: "home-assistant-api-key", KeySource: "user home assistant instance", SkillsUsing: []string{"home-assistant"}, RateLimit: "self-hosted", CostModel: "free"},
	}
}

func NoAuthServiceRegistry() map[string]NoAuthServiceConfig {
	return map[string]NoAuthServiceConfig{
		"bluesky": {
			ServiceKey:  "bluesky",
			SkillsUsing: []string{"bluesky"},
			Notes:       "app-password auth; encrypted in oauth token store with service=bluesky",
		},
		"yahoo-finance": {
			ServiceKey:  "yahoo-finance",
			SkillsUsing: []string{"yahoo-finance", "financial-market-analysis"},
			Notes:       "no api key required; public market data",
		},
		"lastfm": {
			ServiceKey:  "lastfm",
			SkillsUsing: []string{"lastfm"},
			Notes:       "free api key with generous limits",
		},
		"trakt": {
			ServiceKey:  "trakt",
			SkillsUsing: []string{"trakt"},
			Notes:       "client-id flow",
		},
		"local-macos": {
			ServiceKey:  "local-macos",
			SkillsUsing: []string{"apple-remind-me", "apple-mail", "apple-notes-skill", "things-mac", "obsidian"},
			Notes:       "no remote oauth; edge agent handles local permissions",
		},
		"internal-llm-only": {
			ServiceKey:  "internal-llm-only",
			SkillsUsing: []string{"focus-mode", "thinking-partner", "self-improvement"},
			Notes:       "no external API dependencies",
		},
	}
}

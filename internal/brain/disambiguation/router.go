package disambiguation

import (
	"fmt"
	"os"
	"regexp"
	"slices"
	"strings"

	"gopkg.in/yaml.v3"
)

var austrianPostTrackingPattern = regexp.MustCompile(`^[A-Z]{2}[0-9]{9}AT$`)
var upuTrackingPattern = regexp.MustCompile(`^[A-Z]{2}[0-9]{9}[A-Z]{2}$`)

var requiredGroups = []string{
	"apple-notes",
	"notion",
	"spotify",
	"flight-tracking",
	"healthkit",
	"apple-mail",
	"email-send",
	"expense-tracking",
	"package-tracking",
	"places-location",
	"youtube",
}

type Config struct {
	Version int    `yaml:"version"`
	Rules   []Rule `yaml:"rules"`
}

type Rule struct {
	Group           string            `yaml:"group"`
	Canonical       string            `yaml:"canonical"`
	Aliases         []string          `yaml:"aliases"`
	Fallback        string            `yaml:"fallback"`
	Cloud           string            `yaml:"cloud"`
	LocalMac        string            `yaml:"local_mac"`
	Terminal        string            `yaml:"terminal"`
	Analytics       string            `yaml:"analytics"`
	Track           string            `yaml:"track"`
	Find            string            `yaml:"find"`
	FreeTier        string            `yaml:"free_tier"`
	Crud            string            `yaml:"crud"`
	Search          string            `yaml:"search"`
	ByPreference    map[string]string `yaml:"by_preference"`
	Delegates       []string          `yaml:"delegates"`
	International   string            `yaml:"international"`
	Carriers17Track string            `yaml:"carriers_17track"`
	AustrianPost    string            `yaml:"austrian_post"`
	Navigate        string            `yaml:"navigate"`
	NearMe          string            `yaml:"near_me"`
	FindAll         string            `yaml:"find_all"`
	SimpleNearby    string            `yaml:"simple_nearby"`
	Summarize       string            `yaml:"summarize"`
	Download        string            `yaml:"download"`
}

type UserPreferences struct {
	EmailProvider string
	MusicProvider string
}

type ResolveInput struct {
	Group          string
	Text           string
	DeploymentMode string
	UserTier       string
	TrackingNumber string
	Carrier        string
	PreferFallback bool
	Preferences    UserPreferences
}

type ResolveResult struct {
	Group   string
	SkillID string
	Reason  string
}

type Router struct {
	config Config
	rules  map[string]Rule
}

func LoadFromFile(path string) (*Router, error) {
	body, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read disambiguation config: %w", err)
	}
	return LoadFromBytes(body)
}

func LoadFromBytes(body []byte) (*Router, error) {
	var cfg Config
	if err := yaml.Unmarshal(body, &cfg); err != nil {
		return nil, fmt.Errorf("unmarshal disambiguation config: %w", err)
	}
	if cfg.Version != 1 {
		return nil, fmt.Errorf("unsupported disambiguation config version %d", cfg.Version)
	}

	rules := make(map[string]Rule, len(cfg.Rules))
	for _, rule := range cfg.Rules {
		group := strings.ToLower(strings.TrimSpace(rule.Group))
		if group == "" {
			return nil, fmt.Errorf("rule with empty group")
		}
		if _, exists := rules[group]; exists {
			return nil, fmt.Errorf("duplicate disambiguation group %q", group)
		}
		rules[group] = rule
	}

	for _, group := range requiredGroups {
		if _, ok := rules[group]; !ok {
			return nil, fmt.Errorf("missing disambiguation group %q", group)
		}
	}

	return &Router{
		config: cfg,
		rules:  rules,
	}, nil
}

func (r *Router) Version() int {
	return r.config.Version
}

func (r *Router) Groups() []string {
	groups := make([]string, 0, len(r.rules))
	for group := range r.rules {
		groups = append(groups, group)
	}
	slices.Sort(groups)
	return groups
}

func (r *Router) Resolve(input ResolveInput) (ResolveResult, error) {
	group := strings.ToLower(strings.TrimSpace(input.Group))
	rule, ok := r.rules[group]
	if !ok {
		return ResolveResult{}, fmt.Errorf("unknown disambiguation group %q", input.Group)
	}

	text := strings.ToLower(strings.TrimSpace(input.Text))
	deploymentMode := strings.ToLower(strings.TrimSpace(input.DeploymentMode))
	userTier := strings.ToLower(strings.TrimSpace(input.UserTier))
	emailProvider := strings.ToLower(strings.TrimSpace(input.Preferences.EmailProvider))
	carrier := strings.ToLower(strings.TrimSpace(input.Carrier))
	trackingNumber := strings.ToUpper(strings.TrimSpace(input.TrackingNumber))

	switch group {
	case "apple-notes":
		return ResolveResult{Group: group, SkillID: rule.Canonical, Reason: "apple-notes canonical alias"}, nil
	case "notion":
		if input.PreferFallback && rule.Fallback != "" {
			return ResolveResult{Group: group, SkillID: rule.Fallback, Reason: "explicit fallback requested"}, nil
		}
		return ResolveResult{Group: group, SkillID: rule.Canonical, Reason: "notion canonical route"}, nil
	case "spotify":
		if containsAny(text, "history", "top artist", "top track", "analytics", "stats") && rule.Analytics != "" {
			return ResolveResult{Group: group, SkillID: rule.Analytics, Reason: "spotify analytics intent"}, nil
		}
		switch deploymentMode {
		case "local_mac":
			return ResolveResult{Group: group, SkillID: rule.LocalMac, Reason: "local mac deployment"}, nil
		case "terminal":
			return ResolveResult{Group: group, SkillID: rule.Terminal, Reason: "terminal deployment"}, nil
		default:
			return ResolveResult{Group: group, SkillID: rule.Cloud, Reason: "cloud/default deployment"}, nil
		}
	case "flight-tracking":
		if userTier == "free" {
			return ResolveResult{Group: group, SkillID: rule.FreeTier, Reason: "free tier routing"}, nil
		}
		if containsAny(text, "find flight", "find a flight", "search flight", "book flight", "cheapest flight") {
			return ResolveResult{Group: group, SkillID: rule.Find, Reason: "flight discovery intent"}, nil
		}
		return ResolveResult{Group: group, SkillID: rule.Track, Reason: "flight tracking intent"}, nil
	case "healthkit":
		return ResolveResult{Group: group, SkillID: rule.Canonical, Reason: "healthkit canonical alias"}, nil
	case "apple-mail":
		if containsAny(text, "search", "find email", "look up mail") {
			return ResolveResult{Group: group, SkillID: rule.Search, Reason: "apple-mail explicit search intent"}, nil
		}
		return ResolveResult{Group: group, SkillID: rule.Crud, Reason: "apple-mail default CRUD intent"}, nil
	case "email-send":
		if skillID, ok := rule.ByPreference[emailProvider]; ok {
			return ResolveResult{Group: group, SkillID: skillID, Reason: "email provider preference"}, nil
		}
		if skillID, ok := rule.ByPreference["send_only"]; ok {
			return ResolveResult{Group: group, SkillID: skillID, Reason: "email send-only default"}, nil
		}
		return ResolveResult{Group: group, SkillID: rule.Crud, Reason: "email generic fallback"}, nil
	case "expense-tracking":
		return ResolveResult{Group: group, SkillID: rule.Canonical, Reason: "expense orchestrator required"}, nil
	case "package-tracking":
		if austrianPostTrackingPattern.MatchString(trackingNumber) || containsAny(carrier, "austrian", "post.at") {
			return ResolveResult{Group: group, SkillID: rule.AustrianPost, Reason: "austrian post format"}, nil
		}
		if containsAny(carrier, "17track", "yunexpress", "yanwen", "cainiao") || upuTrackingPattern.MatchString(trackingNumber) {
			return ResolveResult{Group: group, SkillID: rule.Carriers17Track, Reason: "17track compatible format"}, nil
		}
		return ResolveResult{Group: group, SkillID: rule.International, Reason: "international/default package tracking"}, nil
	case "places-location":
		if containsAny(text, "navigate", "directions", "route to", "drive to", "walk to") {
			return ResolveResult{Group: group, SkillID: rule.Navigate, Reason: "navigation intent"}, nil
		}
		if containsAny(text, "find all", "all places", "every place", "all restaurants") {
			return ResolveResult{Group: group, SkillID: rule.FindAll, Reason: "exhaustive search intent"}, nil
		}
		if containsAny(text, "near me", "nearby", "closest") {
			if containsAny(text, "quick", "simple") {
				return ResolveResult{Group: group, SkillID: rule.SimpleNearby, Reason: "simple nearby search intent"}, nil
			}
			return ResolveResult{Group: group, SkillID: rule.NearMe, Reason: "rich nearby search intent"}, nil
		}
		return ResolveResult{Group: group, SkillID: rule.SimpleNearby, Reason: "default nearby fallback"}, nil
	case "youtube":
		if containsAny(text, "summarize", "summary", "tl;dr") {
			return ResolveResult{Group: group, SkillID: rule.Summarize, Reason: "youtube summary intent"}, nil
		}
		if containsAny(text, "download", "transcript", "subtitle", "captions", "full video") {
			return ResolveResult{Group: group, SkillID: rule.Download, Reason: "youtube download/transcript intent"}, nil
		}
		return ResolveResult{Group: group, SkillID: rule.Search, Reason: "youtube search/default intent"}, nil
	default:
		return ResolveResult{}, fmt.Errorf("unsupported disambiguation group %q", group)
	}
}

func containsAny(s string, candidates ...string) bool {
	for _, candidate := range candidates {
		if strings.Contains(s, candidate) {
			return true
		}
	}
	return false
}

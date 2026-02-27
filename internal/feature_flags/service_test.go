package feature_flags

import "testing"

func TestFlagCRUD(t *testing.T) {
	t.Parallel()

	s := NewService()
	s.UpsertFlag(Flag{Key: "new_ui", FlagType: "boolean", Enabled: true})
	s.UpsertFlag(Flag{Key: "beta_search", FlagType: "boolean", Enabled: false})

	flags := s.ListFlags()
	if len(flags) != 2 {
		t.Fatalf("unexpected flag count: %d", len(flags))
	}

	flag, ok := s.GetFlag("new_ui")
	if !ok {
		t.Fatal("expected new_ui flag to exist")
	}
	if !flag.Enabled {
		t.Fatal("expected new_ui to be enabled")
	}

	s.DeleteFlag("beta_search")
	if _, ok := s.GetFlag("beta_search"); ok {
		t.Fatal("expected beta_search to be deleted")
	}
}

func TestEvaluateWithRulesAndKillSwitch(t *testing.T) {
	t.Parallel()

	s := NewService()
	s.UpsertFlag(Flag{Key: "context_expansion", FlagType: "ruleset", Enabled: true})
	s.SetRules("context_expansion", []Rule{
		{MatchType: "workspace", MatchValue: "ws_deny", Enabled: false},
		{MatchType: "workspace", MatchValue: "ws_allow", Enabled: true},
	})

	enabled, reason := s.Evaluate("context_expansion", map[string]string{"workspace": "ws_allow"})
	if !enabled || reason != "FEATURE_RULE_MATCH_ALLOW" {
		t.Fatalf("unexpected evaluate allow result: enabled=%v reason=%s", enabled, reason)
	}

	enabled, reason = s.Evaluate("context_expansion", map[string]string{"workspace": "ws_deny"})
	if enabled || reason != "FEATURE_RULE_MATCH_DENY" {
		t.Fatalf("unexpected evaluate deny result: enabled=%v reason=%s", enabled, reason)
	}

	enabled, reason = s.Evaluate("context_expansion", map[string]string{"workspace": "ws_default"})
	if !enabled || reason != "FEATURE_ENABLED_DEFAULT" {
		t.Fatalf("unexpected evaluate default result: enabled=%v reason=%s", enabled, reason)
	}

	s.SetKillSwitch(true)
	enabled, reason = s.Evaluate("context_expansion", map[string]string{"workspace": "ws_allow"})
	if enabled || reason != "FEATURE_DISABLED_BY_KILL_SWITCH" {
		t.Fatalf("unexpected kill switch result: enabled=%v reason=%s", enabled, reason)
	}
}

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

func TestEvaluateForWorkspaceSchemaShape(t *testing.T) {
	t.Parallel()

	s := NewService()
	s.UpsertFlag(Flag{Key: "streaming_ack", FlagType: "boolean", Enabled: true})
	s.SetRules("streaming_ack", []Rule{
		{MatchType: "workspace", MatchValue: "ws_a", Enabled: true, Variant: "canary"},
	})

	result := s.EvaluateForWorkspace("streaming_ack", "", map[string]string{"workspace": "ws_a"})
	if result.FlagKey != "streaming_ack" {
		t.Fatalf("unexpected flag key: %s", result.FlagKey)
	}
	if result.WorkspaceID != "ws_a" {
		t.Fatalf("unexpected workspace id: %s", result.WorkspaceID)
	}
	if !result.Enabled {
		t.Fatalf("expected enabled result: %+v", result)
	}
	if result.Variant != "canary" {
		t.Fatalf("unexpected variant: %s", result.Variant)
	}
	if result.Reason != "FEATURE_RULE_MATCH_ALLOW" {
		t.Fatalf("unexpected reason: %s", result.Reason)
	}
}

func TestEvaluateCacheInvalidatesOnPolicyChange(t *testing.T) {
	t.Parallel()

	s := NewService()
	s.UpsertFlag(Flag{Key: "context_expansion", FlagType: "boolean", Enabled: true})

	initial := s.EvaluateForWorkspace("context_expansion", "ws_cache", map[string]string{"workspace": "ws_cache"})
	if !initial.Enabled {
		t.Fatalf("expected enabled result: %+v", initial)
	}

	s.SetKillSwitch(true)
	afterKillSwitch := s.EvaluateForWorkspace("context_expansion", "ws_cache", map[string]string{"workspace": "ws_cache"})
	if afterKillSwitch.Enabled {
		t.Fatalf("expected disabled by kill switch: %+v", afterKillSwitch)
	}
	if afterKillSwitch.Reason != "FEATURE_DISABLED_BY_KILL_SWITCH" {
		t.Fatalf("unexpected reason after kill switch: %s", afterKillSwitch.Reason)
	}
}

func TestBootstrapSystemFlags(t *testing.T) {
	t.Parallel()

	s := NewService()
	s.BootstrapSystemFlags()

	flags := s.ListFlags()
	if len(flags) != 3 {
		t.Fatalf("unexpected system flag count: got=%d want=3", len(flags))
	}

	check := map[string]bool{
		FlagSkillsRollout:     false,
		FlagLLMProviderSwitch: false,
		FlagCanaryFeatures:    false,
	}
	for _, flag := range flags {
		_, ok := check[flag.Key]
		if !ok {
			t.Fatalf("unexpected system flag key: %s", flag.Key)
		}
		check[flag.Key] = true
	}

	for key, seen := range check {
		if !seen {
			t.Fatalf("missing expected system flag key: %s", key)
		}
	}
}

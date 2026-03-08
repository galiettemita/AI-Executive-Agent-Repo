package admin

import "testing"

func TestSkillACLOverrideSetAndCheck(t *testing.T) {
	t.Parallel()
	svc := NewSkillACLOverrideService()

	override, err := svc.SetOverride(SkillACLOverride{
		WorkspaceID: "ws1",
		UserID:      "u1",
		SkillID:     "skill_finance",
		Allowed:     false,
		Reason:      "compliance restriction",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if override.ID == "" {
		t.Fatal("expected generated ID")
	}

	allowed, hasOverride := svc.IsSkillAllowed("ws1", "u1", "skill_finance")
	if allowed {
		t.Fatal("expected skill to be denied")
	}
	if !hasOverride {
		t.Fatal("expected override to exist")
	}
}

func TestSkillACLOverrideDefaultAllow(t *testing.T) {
	t.Parallel()
	svc := NewSkillACLOverrideService()

	allowed, hasOverride := svc.IsSkillAllowed("ws1", "u1", "skill_any")
	if !allowed {
		t.Fatal("expected default allow")
	}
	if hasOverride {
		t.Fatal("expected no override")
	}
}

func TestSkillACLOverrideMissingFields(t *testing.T) {
	t.Parallel()
	svc := NewSkillACLOverrideService()

	_, err := svc.SetOverride(SkillACLOverride{UserID: "u1", SkillID: "s1"})
	if err == nil {
		t.Fatal("expected error for missing workspace_id")
	}
	_, err = svc.SetOverride(SkillACLOverride{WorkspaceID: "ws1", SkillID: "s1"})
	if err == nil {
		t.Fatal("expected error for missing user_id")
	}
	_, err = svc.SetOverride(SkillACLOverride{WorkspaceID: "ws1", UserID: "u1"})
	if err == nil {
		t.Fatal("expected error for missing skill_id")
	}
}

func TestSkillACLOverrideRemove(t *testing.T) {
	t.Parallel()
	svc := NewSkillACLOverrideService()

	_, _ = svc.SetOverride(SkillACLOverride{
		WorkspaceID: "ws1",
		UserID:      "u1",
		SkillID:     "s1",
		Allowed:     false,
	})

	err := svc.RemoveOverride("ws1", "u1", "s1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	allowed, hasOverride := svc.IsSkillAllowed("ws1", "u1", "s1")
	if !allowed || hasOverride {
		t.Fatal("expected default allow after removal")
	}
}

func TestSkillACLOverrideRemoveNotFound(t *testing.T) {
	t.Parallel()
	svc := NewSkillACLOverrideService()
	err := svc.RemoveOverride("ws1", "u1", "nonexistent")
	if err == nil {
		t.Fatal("expected error for not found")
	}
}

func TestSkillACLOverrideGetUserOverrides(t *testing.T) {
	t.Parallel()
	svc := NewSkillACLOverrideService()

	_, _ = svc.SetOverride(SkillACLOverride{
		WorkspaceID: "ws1", UserID: "u1", SkillID: "s1", Allowed: false,
	})
	_, _ = svc.SetOverride(SkillACLOverride{
		WorkspaceID: "ws1", UserID: "u1", SkillID: "s2", Allowed: true,
	})
	_, _ = svc.SetOverride(SkillACLOverride{
		WorkspaceID: "ws1", UserID: "u2", SkillID: "s1", Allowed: false,
	})

	overrides := svc.GetUserOverrides("ws1", "u1")
	if len(overrides) != 2 {
		t.Fatalf("expected 2 overrides for u1, got %d", len(overrides))
	}
}

func TestSkillACLOverrideUpdate(t *testing.T) {
	t.Parallel()
	svc := NewSkillACLOverrideService()

	_, _ = svc.SetOverride(SkillACLOverride{
		WorkspaceID: "ws1", UserID: "u1", SkillID: "s1", Allowed: false,
	})
	_, _ = svc.SetOverride(SkillACLOverride{
		WorkspaceID: "ws1", UserID: "u1", SkillID: "s1", Allowed: true, Reason: "approved",
	})

	allowed, _ := svc.IsSkillAllowed("ws1", "u1", "s1")
	if !allowed {
		t.Fatal("expected skill to be allowed after update")
	}
}

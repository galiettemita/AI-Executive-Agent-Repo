package memory

import (
	"context"
	"math"
	"strings"
	"testing"
)

// Decay model tests

func TestInitialTransferConfidence_AppliesMultiplier(t *testing.T) {
	result := InitialTransferConfidence(0.90)
	expected := 0.90 * 0.70
	if math.Abs(result-expected) > 0.001 {
		t.Fatalf("got %v, want %v", result, expected)
	}
}

func TestDecayedTransferConfidence_ZeroObservations_Unchanged(t *testing.T) {
	result := DecayedTransferConfidence(0.63, 0)
	if math.Abs(result-0.63) > 0.001 {
		t.Fatalf("got %v, want 0.63", result)
	}
}

func TestDecayedTransferConfidence_FewObservations_Partial(t *testing.T) {
	result := DecayedTransferConfidence(0.63, 3)
	expected := 0.63 * (1.0 - 3*0.12) // 0.63 × 0.64 ≈ 0.4032
	if math.Abs(result-expected) > 0.01 {
		t.Fatalf("got %v, want ~%v", result, expected)
	}
}

func TestDecayedTransferConfidence_ManyObservations_FlooredAtMin(t *testing.T) {
	result := DecayedTransferConfidence(0.63, 10)
	expected := 0.63 * MinTransferDecayFactor // 0.63 × 0.10 = 0.063
	if math.Abs(result-expected) > 0.01 {
		t.Fatalf("got %v, want ~%v", result, expected)
	}
}

func TestShouldUseTransfer_LocalStrong_ReturnsFalse(t *testing.T) {
	if ShouldUseTransfer(0.60, 0.80) {
		t.Fatal("should return false when local is strong")
	}
}

func TestShouldUseTransfer_LocalWeak_TransferStrong_ReturnsTrue(t *testing.T) {
	if !ShouldUseTransfer(0.50, 0.20) {
		t.Fatal("should return true when local is weak and transfer is strong")
	}
}

func TestShouldUseTransfer_BothWeak_ReturnsFalse(t *testing.T) {
	if ShouldUseTransfer(0.20, 0.10) {
		t.Fatal("should return false when transfer confidence <= 0.30")
	}
}

// Transfer service tests

type mockTransferRepo struct {
	entries      []PreferenceTransferIndexEntry
	localObs     int
	logCalled    bool
}

func (m *mockTransferRepo) Upsert(_ context.Context, _ PreferenceTransferIndexEntry) error {
	return nil
}

func (m *mockTransferRepo) FindForUser(_ context.Context, _ string, _ []float32, _ int) ([]PreferenceTransferIndexEntry, error) {
	return m.entries, nil
}

func (m *mockTransferRepo) GetLocalObservationCount(_ context.Context, _, _ string) (int, error) {
	return m.localObs, nil
}

func (m *mockTransferRepo) LogTransfer(_ context.Context, _, _, _ string, _ float64) error {
	m.logCalled = true
	return nil
}

type mockWSSettings struct {
	settings TransferSettings
	ownerID  string
}

func (m *mockWSSettings) GetTransferSettings(_ context.Context, _ string) (TransferSettings, error) {
	return m.settings, nil
}

func (m *mockWSSettings) GetOwnerID(_ context.Context, _ string) (string, error) {
	return m.ownerID, nil
}

type mockTransferEmbedder struct{}

func (m *mockTransferEmbedder) Embed(_ context.Context, texts []string) ([][]float32, error) {
	result := make([][]float32, len(texts))
	for i := range texts {
		v := make([]float32, 8)
		v[0] = 0.9
		result[i] = v
	}
	return result, nil
}

type nopTransferLogger struct{}

func (nopTransferLogger) Info(string, ...any)  {}
func (nopTransferLogger) Warn(string, ...any)  {}
func (nopTransferLogger) Error(string, ...any) {}

func TestQueryTransferredPreferences_TransferDisabled_ReturnsNil(t *testing.T) {
	svc := NewPreferenceTransferService(
		&mockTransferRepo{},
		&mockWSSettings{settings: TransferSettings{Enabled: false, Scope: "none"}, ownerID: "u1"},
		&mockTransferEmbedder{},
		nopTransferLogger{},
	)

	prefs, err := svc.QueryTransferredPreferences(context.Background(), "ws-1", "test", 5)
	if err != nil {
		t.Fatal(err)
	}
	if prefs != nil {
		t.Fatal("expected nil when transfer disabled")
	}
}

func TestQueryTransferredPreferences_FiltersSameWorkspace(t *testing.T) {
	svc := NewPreferenceTransferService(
		&mockTransferRepo{entries: []PreferenceTransferIndexEntry{
			{ID: "e1", SourceWorkspaceID: "ws-1", PreferenceCategory: "scheduling", PreferenceSummary: "prefers mornings", Confidence: 0.9},
		}},
		&mockWSSettings{settings: TransferSettings{Enabled: true, Scope: "all"}, ownerID: "u1"},
		&mockTransferEmbedder{},
		nopTransferLogger{},
	)

	prefs, err := svc.QueryTransferredPreferences(context.Background(), "ws-1", "meeting", 5)
	if err != nil {
		t.Fatal(err)
	}
	if len(prefs) != 0 {
		t.Fatal("should not return preferences from same workspace")
	}
}

func TestQueryTransferredPreferences_AppliesDecay(t *testing.T) {
	svc := NewPreferenceTransferService(
		&mockTransferRepo{
			entries: []PreferenceTransferIndexEntry{
				{ID: "e1", SourceWorkspaceID: "ws-2", PreferenceCategory: "style", PreferenceSummary: "formal tone", Confidence: 0.95},
			},
			localObs: 2, // 2 local observations: initial=0.95*0.7=0.665, decay=0.665*(1-2*0.12)=0.665*0.76≈0.505
		},
		&mockWSSettings{settings: TransferSettings{Enabled: true, Scope: "all"}, ownerID: "u1"},
		&mockTransferEmbedder{},
		nopTransferLogger{},
	)

	prefs, err := svc.QueryTransferredPreferences(context.Background(), "ws-1", "email", 5)
	if err != nil {
		t.Fatal(err)
	}
	if len(prefs) == 0 {
		t.Fatal("expected at least one preference (decayed but still above 0.30)")
	}
	if prefs[0].EffectiveConfidence >= 0.665 {
		t.Fatalf("expected decayed confidence < 0.665, got %v", prefs[0].EffectiveConfidence)
	}
}

func TestQueryTransferredPreferences_LowEffectiveConfidence_Filtered(t *testing.T) {
	svc := NewPreferenceTransferService(
		&mockTransferRepo{
			entries: []PreferenceTransferIndexEntry{
				{ID: "e1", SourceWorkspaceID: "ws-2", PreferenceCategory: "style", PreferenceSummary: "formal", Confidence: 0.5},
			},
			localObs: 5, // heavy local learning → decayed below 0.30
		},
		&mockWSSettings{settings: TransferSettings{Enabled: true, Scope: "all"}, ownerID: "u1"},
		&mockTransferEmbedder{},
		nopTransferLogger{},
	)

	prefs, err := svc.QueryTransferredPreferences(context.Background(), "ws-1", "email", 5)
	if err != nil {
		t.Fatal(err)
	}
	if len(prefs) != 0 {
		t.Fatalf("expected 0 preferences (decayed below threshold), got %d", len(prefs))
	}
}

func TestQueryTransferredPreferences_HighConfidence_Returned(t *testing.T) {
	svc := NewPreferenceTransferService(
		&mockTransferRepo{
			entries: []PreferenceTransferIndexEntry{
				{ID: "e1", SourceWorkspaceID: "ws-2", PreferenceCategory: "style", PreferenceSummary: "formal tone", Confidence: 0.95},
			},
			localObs: 0, // no local learning → full transfer confidence
		},
		&mockWSSettings{settings: TransferSettings{Enabled: true, Scope: "all"}, ownerID: "u1"},
		&mockTransferEmbedder{},
		nopTransferLogger{},
	)

	prefs, err := svc.QueryTransferredPreferences(context.Background(), "ws-1", "email", 5)
	if err != nil {
		t.Fatal(err)
	}
	if len(prefs) != 1 {
		t.Fatalf("expected 1 preference, got %d", len(prefs))
	}
	if prefs[0].Summary != "formal tone" {
		t.Fatalf("wrong summary: %s", prefs[0].Summary)
	}
}

// Format tests

func TestFormat_EmptyPrefs_EmptyString(t *testing.T) {
	result := FormatTransferredPreferencesForContext(nil)
	if result != "" {
		t.Fatalf("expected empty, got %q", result)
	}
}

func TestFormat_LowConfidence_UsesTendsto(t *testing.T) {
	result := FormatTransferredPreferencesForContext([]TransferredPreference{
		{Summary: "prefers mornings", EffectiveConfidence: 0.40},
	})
	if !strings.Contains(result, "typically") {
		t.Fatalf("expected 'typically' for low confidence, got: %s", result)
	}
}

func TestFormat_HighConfidence_UsesConsistently(t *testing.T) {
	result := FormatTransferredPreferencesForContext([]TransferredPreference{
		{Summary: "formal tone", EffectiveConfidence: 0.70},
	})
	if !strings.Contains(result, "consistently") {
		t.Fatalf("expected 'consistently' for high confidence, got: %s", result)
	}
}

func TestFormat_NeverExposesSourceWorkspaceID(t *testing.T) {
	result := FormatTransferredPreferencesForContext([]TransferredPreference{
		{Summary: "prefers mornings", SourceWorkspaceID: "ws-secret-123", EffectiveConfidence: 0.60},
	})
	if strings.Contains(result, "ws-secret-123") {
		t.Fatal("SourceWorkspaceID must NEVER appear in formatted context")
	}
}

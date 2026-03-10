package contracts

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

type blueprintManifest struct {
	Meta       manifestMeta      `json:"_meta"`
	Blueprints []blueprintEntry  `json:"blueprints"`
}

type manifestMeta struct {
	Title   string `json:"title"`
	Version string `json:"version"`
}

type blueprintEntry struct {
	BlueprintID   string `json:"blueprint_id"`
	Label         string `json:"label"`
	RepoPath      string `json:"repo_path"`
	SHA256        string `json:"sha256"`
	AuthorityRank int    `json:"authority_rank"`
	VersionScope  string `json:"version_scope"`
}

// TestBlueprintManifestCompleteness asserts that the manifest lists exactly
// the 7 mandatory blueprints with the correct IDs.
func TestBlueprintManifestCompleteness(t *testing.T) {
	t.Parallel()

	manifest := loadBlueprintManifest(t)

	if len(manifest.Blueprints) != 7 {
		t.Fatalf("blueprint manifest must list exactly 7 mandatory blueprints, got %d", len(manifest.Blueprints))
	}

	requiredIDs := map[string]bool{
		"BP06": false,
		"BP07": false,
		"BP08": false,
		"BP09": false,
		"BP11": false,
		"BP16": false,
		"BP17": false,
	}

	for _, bp := range manifest.Blueprints {
		if _, ok := requiredIDs[bp.BlueprintID]; !ok {
			t.Fatalf("unexpected blueprint_id in manifest: %s", bp.BlueprintID)
		}
		if requiredIDs[bp.BlueprintID] {
			t.Fatalf("duplicate blueprint_id in manifest: %s", bp.BlueprintID)
		}
		requiredIDs[bp.BlueprintID] = true

		if bp.RepoPath == "" {
			t.Fatalf("blueprint %s missing repo_path", bp.BlueprintID)
		}
		if bp.SHA256 == "" {
			t.Fatalf("blueprint %s missing sha256", bp.BlueprintID)
		}
		if bp.AuthorityRank < 1 || bp.AuthorityRank > 7 {
			t.Fatalf("blueprint %s has invalid authority_rank: %d", bp.BlueprintID, bp.AuthorityRank)
		}
	}

	for id, found := range requiredIDs {
		if !found {
			t.Fatalf("mandatory blueprint missing from manifest: %s", id)
		}
	}
}

// TestBlueprintManifestHashIntegrity verifies that the sha256 recorded in the
// manifest matches the actual file content for every mandatory blueprint.
func TestBlueprintManifestHashIntegrity(t *testing.T) {
	t.Parallel()

	root := repositoryRoot(t)
	manifest := loadBlueprintManifest(t)

	for _, bp := range manifest.Blueprints {
		filePath := filepath.Join(root, bp.RepoPath)
		data, err := os.ReadFile(filePath)
		if err != nil {
			t.Fatalf("blueprint %s (%s): file not found: %v", bp.BlueprintID, bp.RepoPath, err)
		}

		hash := sha256.Sum256(data)
		actual := hex.EncodeToString(hash[:])
		if actual != bp.SHA256 {
			t.Fatalf("blueprint %s hash mismatch: manifest=%s actual=%s", bp.BlueprintID, bp.SHA256, actual)
		}
	}
}

func loadBlueprintManifest(t *testing.T) blueprintManifest {
	t.Helper()

	root := repositoryRoot(t)
	path := filepath.Join(root, "docs", "BLUEPRINT_MANIFEST.json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read blueprint manifest: %v", err)
	}

	var manifest blueprintManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		t.Fatalf("parse blueprint manifest: %v", err)
	}
	return manifest
}

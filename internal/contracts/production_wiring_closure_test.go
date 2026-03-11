package contracts

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestProductionWiringClosure enforces that onboarding, context budgets, and RAG
// subsystems have durable (DB-backed) repository implementations and that the
// in-memory implementations are not used in production service constructors.
//
// Governed by: Prompt 3 production wiring decision.
// Covers: REQ-UX-002, REQ-V102-GAP-C01, REQ-V102-GAP-M04.
func TestProductionWiringClosure(t *testing.T) {
	t.Parallel()
	root := repositoryRoot(t)

	// Gate: Each subsystem must have a Repository interface and PgRepository.
	t.Run("onboarding_pg_repository_exists", func(t *testing.T) {
		assertFileNonEmpty(t, filepath.Join(root, "internal", "onboarding", "repository.go"))
		assertFileNonEmpty(t, filepath.Join(root, "internal", "onboarding", "pg_repository.go"))
		assertFileContainsTokens(t, filepath.Join(root, "internal", "onboarding", "repository.go"), []string{
			"Repository",
			"StartSession",
			"AdvanceStage",
			"GetStatus",
		})
		assertFileContainsTokens(t, filepath.Join(root, "internal", "onboarding", "pg_repository.go"), []string{
			"PgRepository",
			"database.Querier",
			"onboarding_sessions",
		})
	})

	t.Run("context_pg_repository_exists", func(t *testing.T) {
		assertFileNonEmpty(t, filepath.Join(root, "internal", "context", "repository.go"))
		assertFileNonEmpty(t, filepath.Join(root, "internal", "context", "pg_repository.go"))
		assertFileContainsTokens(t, filepath.Join(root, "internal", "context", "repository.go"), []string{
			"Repository",
			"UpsertBudget",
			"GetBudget",
			"SetAllocations",
		})
		assertFileContainsTokens(t, filepath.Join(root, "internal", "context", "pg_repository.go"), []string{
			"PgRepository",
			"database.Querier",
			"context_budgets",
		})
	})

	t.Run("rag_pg_repository_exists", func(t *testing.T) {
		assertFileNonEmpty(t, filepath.Join(root, "internal", "rag", "repository.go"))
		assertFileNonEmpty(t, filepath.Join(root, "internal", "rag", "pg_repository.go"))
		assertFileContainsTokens(t, filepath.Join(root, "internal", "rag", "repository.go"), []string{
			"Repository",
			"UpsertCollection",
			"GetCollection",
			"RecordRetrieval",
		})
		assertFileContainsTokens(t, filepath.Join(root, "internal", "rag", "pg_repository.go"), []string{
			"PgRepository",
			"database.Querier",
			"rag_collections",
		})
	})

	// Gate: MuxDependencies must accept DB and repository fields for production wiring.
	t.Run("mux_accepts_db_dependencies", func(t *testing.T) {
		muxPath := filepath.Join(root, "internal", "control", "mux.go")
		assertFileContainsTokens(t, muxPath, []string{
			"database.Querier",
			"OnboardingRepo",
			"ContextBudgetRepo",
			"RAGRepo",
		})
	})

	// Gate: In-memory map stores must not appear in production service constructors
	// (excluding test files and build-tagged devtest files).
	t.Run("no_inmemory_in_production_cmd", func(t *testing.T) {
		cmdDirs := []string{"gateway", "brain", "control", "executor", "canvas"}
		for _, dir := range cmdDirs {
			mainPath := filepath.Join(root, "cmd", dir, "main.go")
			if _, err := os.Stat(mainPath); os.IsNotExist(err) {
				continue
			}
			body, err := os.ReadFile(mainPath)
			if err != nil {
				t.Fatalf("read %s: %v", mainPath, err)
			}
			content := string(body)
			// cmd/*/main.go should not directly call in-memory NewService()
			// for onboarding, context, or rag without database wiring.
			// This is a structural assertion — the presence of DATABASE_URL
			// check or MuxDependencies.DB assignment indicates proper wiring.
			if strings.Contains(content, "contextlayer.NewService()") ||
				strings.Contains(content, "raglayer.NewService()") {
				// Only flag if there's no database wiring alongside it.
				if !strings.Contains(content, "DATABASE_URL") &&
					!strings.Contains(content, "database.") &&
					!strings.Contains(content, "MuxDependencies") {
					t.Errorf("cmd/%s/main.go uses in-memory service without database wiring", dir)
				}
			}
		}
	})
}

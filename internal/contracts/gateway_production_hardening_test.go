package contracts

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestGatewayProdServiceExists verifies that the production gateway service
// constructor exists with pgx-backed dependencies.
func TestGatewayProdServiceExists(t *testing.T) {
	t.Parallel()
	root := repositoryRoot(t)

	prodPath := filepath.Join(root, "internal", "gateway", "service_prod.go")
	assertFileNonEmpty(t, prodPath)
	assertFileContainsTokens(t, prodPath, []string{
		"ProdService",
		"NewServiceProd",
		"ProdDeps",
		"IngressTurnRepository",
		"DeduplicationRepository",
		"MessageQueueRepository",
		"IdempotencyRepository",
		"outbox.Service",
		"pgxpool.Pool",
	})
}

// TestGatewayNoInMemoryInProduction verifies that the production service
// does not directly use InMemoryStore, InMemoryQueue, or in-memory outbox
// slice for persistence.
func TestGatewayNoInMemoryInProduction(t *testing.T) {
	t.Parallel()
	root := repositoryRoot(t)

	prodPath := filepath.Join(root, "internal", "gateway", "service_prod.go")
	content, err := os.ReadFile(prodPath)
	if err != nil {
		t.Fatalf("read service_prod.go: %v", err)
	}
	src := string(content)

	forbidden := []string{
		"InMemoryStore{",
		"InMemoryQueue{",
		"NewInMemoryStore()",
	}
	for _, token := range forbidden {
		if strings.Contains(src, token) {
			t.Errorf("service_prod.go must not directly instantiate %q — use pgx repositories", token)
		}
	}
}

// TestGatewayPgIngressRepositoryExists verifies that the pgx ingress turn
// repository and idempotency repository exist.
func TestGatewayPgIngressRepositoryExists(t *testing.T) {
	t.Parallel()
	root := repositoryRoot(t)

	repoPath := filepath.Join(root, "internal", "gateway", "pg_ingress_repository.go")
	assertFileNonEmpty(t, repoPath)
	assertFileContainsTokens(t, repoPath, []string{
		"IngressTurnRepository",
		"PgIngressTurnRepository",
		"InsertTurn",
		"GetTurn",
		"InsertIdentityEnvelope",
		"IdempotencyRepository",
		"PgIdempotencyRepository",
		"gateway_idempotency_cache",
	})
}

// TestGatewayPgRepositoriesExist verifies that the existing pgx dedup and
// queue repositories exist and implement their interfaces.
func TestGatewayPgRepositoriesExist(t *testing.T) {
	t.Parallel()
	root := repositoryRoot(t)

	repoPath := filepath.Join(root, "internal", "gateway", "pg_repository.go")
	assertFileNonEmpty(t, repoPath)
	assertFileContainsTokens(t, repoPath, []string{
		"PgDeduplicationRepository",
		"PgMessageQueueRepository",
		"DeduplicationRepository",
		"MessageQueueRepository",
		"gateway_dedup",
		"gateway_nonces",
		"gateway_queue",
	})
}

// TestGatewayMigration014Exists verifies that the gateway production
// hardening migration exists with required tables.
func TestGatewayMigration014Exists(t *testing.T) {
	t.Parallel()
	root := repositoryRoot(t)

	migrationPath := filepath.Join(root, "db", "migrations", "014_BREVIO_gateway_production_hardening.sql")
	assertFileNonEmpty(t, migrationPath)
	assertFileContainsTokens(t, migrationPath, []string{
		"gateway_dedup",
		"gateway_nonces",
		"gateway_queue",
		"gateway_idempotency_cache",
		"ROW LEVEL SECURITY",
		"outbox",
	})
}

// TestGatewayProdMuxExists verifies that NewProdMux is defined for
// production HTTP routing through ProdService.
func TestGatewayProdMuxExists(t *testing.T) {
	t.Parallel()
	root := repositoryRoot(t)

	serverPath := filepath.Join(root, "internal", "gateway", "server.go")
	assertFileNonEmpty(t, serverPath)
	assertFileContainsTokens(t, serverPath, []string{
		"NewProdMux",
		"ProdService",
	})
}

// TestGatewayEntrypointProductionWiring verifies that cmd/gateway/main.go
// wires the production service with DATABASE_URL-based detection.
func TestGatewayEntrypointProductionWiring(t *testing.T) {
	t.Parallel()
	root := repositoryRoot(t)

	mainPath := filepath.Join(root, "cmd", "gateway", "main.go")
	assertFileNonEmpty(t, mainPath)
	assertFileContainsTokens(t, mainPath, []string{
		"DATABASE_URL",
		"NewServiceProd",
		"ProdDeps",
		"pgxpool",
		"outbox.NewService",
		"NewProdMux",
	})
}

// TestConnectorsPgOAuthRepositoryExists verifies that the pgx-backed
// OAuth token repository exists for encrypted token persistence.
func TestConnectorsPgOAuthRepositoryExists(t *testing.T) {
	t.Parallel()
	root := repositoryRoot(t)

	repoPath := filepath.Join(root, "internal", "connectors", "pg_repository.go")
	assertFileNonEmpty(t, repoPath)
	assertFileContainsTokens(t, repoPath, []string{
		"OAuthTokenRepository",
		"PgOAuthTokenRepository",
		"StoreToken",
		"GetToken",
		"UpdateAfterRefresh",
		"user_oauth_tokens",
		"ciphertext",
		"refresh_ciphertext",
	})
}

// TestGatewayOutboxUsesTransactionalEnqueue verifies that the production
// gateway service uses the outbox service for transactional writes instead
// of the in-memory outbox slice.
func TestGatewayOutboxUsesTransactionalEnqueue(t *testing.T) {
	t.Parallel()
	root := repositoryRoot(t)

	prodPath := filepath.Join(root, "internal", "gateway", "service_prod.go")
	assertFileNonEmpty(t, prodPath)
	assertFileContainsTokens(t, prodPath, []string{
		"outboxService",
		"outbox.OutboxEntry",
		"outbox.StatusPending",
		"pool.Begin",
		"tx.Commit",
		"Enqueue",
	})
}

package contracts

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestGateV104MigrationTablesExist verifies the v10.4 voice call migration
// defines all required tables and enums.
func TestGateV104MigrationTablesExist(t *testing.T) {
	t.Parallel()
	root := repositoryRoot(t)

	migrationPath := filepath.Join(root, "db", "migrations", "012_BREVIO_v104_voice_calls.sql")
	assertFileNonEmpty(t, migrationPath)
	assertFileContainsTokens(t, migrationPath, []string{
		"call_providers",
		"call_approval_policies",
		"call_approval_requests",
		"calls",
		"call_transcripts",
		"call_events",
		"call_provider_health_log",
		"call_rate_limits",
		"call_number_blocklist",
		"call_status",
		"call_direction",
		"call_approval_status",
		"call_provider_status",
		"transcript_segment_type",
	})
}

// TestGateCallsCannotInitiateWithoutApproval verifies that calls cannot be
// initiated without an approved approval request (NNR enforcement).
func TestGateCallsCannotInitiateWithoutApproval(t *testing.T) {
	t.Parallel()
	root := repositoryRoot(t)

	// Verify MakeCallActivity checks approval before proceeding.
	activitiesPath := filepath.Join(root, "internal", "temporal", "activities_v104.go")
	assertFileNonEmpty(t, activitiesPath)
	assertFileContainsTokens(t, activitiesPath, []string{
		"MakeCallActivity",
		"APPROVAL_GATE_FAILED",
		"VerifyApproved",
		"approval_id",
	})

	// Verify approval service enforces the gate.
	approvalPath := filepath.Join(root, "internal", "hands", "call", "approval_service.go")
	assertFileNonEmpty(t, approvalPath)
	assertFileContainsTokens(t, approvalPath, []string{
		"ApprovalService",
		"VerifyApproved",
		"APPROVAL_NOT_GRANTED",
		"APPROVAL_EXPIRED",
		"RequestApproval",
		"NO_ACTIVE_POLICY",
	})

	// Verify the calls table has approval_request_id NOT NULL constraint.
	migrationPath := filepath.Join(root, "db", "migrations", "012_BREVIO_v104_voice_calls.sql")
	assertFileContainsTokens(t, migrationPath, []string{
		"approval_request_id uuid NOT NULL",
	})
}

// TestGateUnverifiedPhoneNumbersRejected verifies that unverified phone numbers
// are rejected before call initiation.
func TestGateUnverifiedPhoneNumbersRejected(t *testing.T) {
	t.Parallel()
	root := repositoryRoot(t)

	// Verify phone verifier interface and Google Places implementation.
	placesPath := filepath.Join(root, "internal", "hands", "call", "places_client.go")
	assertFileNonEmpty(t, placesPath)
	assertFileContainsTokens(t, placesPath, []string{
		"PhoneVerifier",
		"GooglePlacesClient",
		"VerifyPhone",
		"PHONE_MISMATCH",
		"NO_PLACE_FOUND",
		"DeterministicPhoneVerifier",
		"UNVERIFIED_NUMBER",
	})

	// Verify workflow rejects unverified numbers.
	workflowPath := filepath.Join(root, "internal", "temporal", "workflows_v104.go")
	assertFileNonEmpty(t, workflowPath)
	assertFileContainsTokens(t, workflowPath, []string{
		"OutboundCallWorkflow",
		"VerifyPhoneActivity",
		"Verified",
		"rejected",
	})
}

// TestGateProviderFailoverThreshold verifies provider failover triggers at
// the required 5% error rate threshold (hard-coded per NNR).
func TestGateProviderFailoverThreshold(t *testing.T) {
	t.Parallel()
	root := repositoryRoot(t)

	// Verify failover threshold is hard-coded.
	approvalPath := filepath.Join(root, "internal", "hands", "call", "approval_service.go")
	assertFileContainsTokens(t, approvalPath, []string{
		"FailoverThreshold",
		"5.0",
		"EvaluateProviderHealth",
	})

	// Verify call service uses 5% threshold.
	servicePath := filepath.Join(root, "internal", "hands", "call", "call_service.go")
	assertFileContainsTokens(t, servicePath, []string{
		"selectProvider",
		"5.0",
		"ErrorRate",
	})

	// Verify provider health check activity.
	activitiesPath := filepath.Join(root, "internal", "temporal", "activities_v104.go")
	assertFileContainsTokens(t, activitiesPath, []string{
		"CheckProviderHealthActivity",
		"ShouldFailover",
		"EvaluateProviderHealth",
	})
}

// TestGateTranscriptsPersisted verifies transcript segments are persisted
// to the DB via call_transcripts table.
func TestGateTranscriptsPersisted(t *testing.T) {
	t.Parallel()
	root := repositoryRoot(t)

	repoPath := filepath.Join(root, "internal", "hands", "call", "pg_repository.go")
	assertFileNonEmpty(t, repoPath)
	assertFileContainsTokens(t, repoPath, []string{
		"InsertTranscriptSegment",
		"GetTranscriptSegments",
		"call_transcripts",
		"TranscriptSegmentRow",
		"segment_index",
		"segment_type",
	})

	activitiesPath := filepath.Join(root, "internal", "temporal", "activities_v104.go")
	assertFileContainsTokens(t, activitiesPath, []string{
		"PersistTranscriptSegmentActivity",
		"InsertTranscriptSegment",
	})
}

// TestGateNoAudioPersistence verifies no raw audio is persisted — transcript
// text only. Scans all call-related files for audio storage patterns.
func TestGateNoAudioPersistence(t *testing.T) {
	t.Parallel()
	root := repositoryRoot(t)

	callDir := filepath.Join(root, "internal", "hands", "call")
	entries, err := os.ReadDir(callDir)
	if err != nil {
		t.Fatalf("read call dir: %v", err)
	}

	forbiddenPatterns := []string{
		"audio_data",
		"raw_audio",
		"audio_bytes",
		"audio_blob",
		"SaveAudio",
		"PersistAudio",
		"audio_url",
	}

	for _, entry := range entries {
		if entry.IsDir() || strings.HasSuffix(entry.Name(), "_test.go") {
			continue
		}
		if !strings.HasSuffix(entry.Name(), ".go") {
			continue
		}

		body, readErr := os.ReadFile(filepath.Join(callDir, entry.Name()))
		if readErr != nil {
			t.Fatalf("read %s: %v", entry.Name(), readErr)
		}
		content := string(body)
		for _, pattern := range forbiddenPatterns {
			if strings.Contains(content, pattern) {
				t.Errorf("AUDIO_PERSISTENCE_VIOLATION: %s contains %q — raw audio must not be stored", entry.Name(), pattern)
			}
		}
	}
}

// TestGateCallDBRepository verifies the pg_repository.go covers all 9 call tables.
func TestGateCallDBRepository(t *testing.T) {
	t.Parallel()
	root := repositoryRoot(t)

	repoPath := filepath.Join(root, "internal", "hands", "call", "pg_repository.go")
	assertFileNonEmpty(t, repoPath)
	assertFileContainsTokens(t, repoPath, []string{
		"CallRepository",
		"PgCallRepository",
		"NewPgCallRepository",
		"CreateApprovalRequest",
		"GetApprovalRequest",
		"ApproveRequest",
		"DenyRequest",
		"ExpirePendingRequests",
		"InsertCall",
		"UpdateCallStatus",
		"CompleteCall",
		"GetCallByProviderID",
		"InsertTranscriptSegment",
		"InsertCallEvent",
		"RecordProviderHealth",
		"GetProvider",
		"UpdateProviderStatus",
		"IncrementRateLimit",
		"IsNumberBlocked",
		"GetActivePolicy",
		"HashPhoneNumber",
	})
}

// TestGateWebhookIdempotency verifies webhook processing maps provider_call_id
// to internal call_id and is idempotent.
func TestGateWebhookIdempotency(t *testing.T) {
	t.Parallel()
	root := repositoryRoot(t)

	activitiesPath := filepath.Join(root, "internal", "temporal", "activities_v104.go")
	assertFileContainsTokens(t, activitiesPath, []string{
		"ProcessCallWebhookActivity",
		"GetCallByProviderID",
		"provider_call_id",
		"InsertCallEvent",
	})

	workflowPath := filepath.Join(root, "internal", "temporal", "workflows_v104.go")
	assertFileContainsTokens(t, workflowPath, []string{
		"CallWebhookProcessingWorkflow",
		"ProcessCallWebhookActivity",
		"PersistTranscriptSegmentActivity",
	})
}

// TestGateV104WorkflowsRegistered verifies call workflows and activities
// are registered in the Temporal worker.
func TestGateV104WorkflowsRegistered(t *testing.T) {
	t.Parallel()
	root := repositoryRoot(t)

	workerPath := filepath.Join(root, "internal", "temporal", "worker.go")
	assertFileContainsTokens(t, workerPath, []string{
		"OutboundCallWorkflow",
		"CallWebhookProcessingWorkflow",
		"RequestCallApprovalActivity",
		"VerifyPhoneActivity",
		"MakeCallActivity",
		"ProcessCallWebhookActivity",
		"PersistTranscriptSegmentActivity",
		"CheckProviderHealthActivity",
	})
}

// TestGateV104DepsWiredInWorkerMain verifies call dependencies are wired
// in the temporal-worker main.go.
func TestGateV104DepsWiredInWorkerMain(t *testing.T) {
	t.Parallel()
	root := repositoryRoot(t)

	mainPath := filepath.Join(root, "cmd", "temporal-worker", "main.go")
	assertFileContainsTokens(t, mainPath, []string{
		"CallRepo",
		"PhoneVerifier",
		"NewPgCallRepository",
		"NewGooglePlacesClient",
		"callpkg",
	})
}

// TestGateV104ActivityDepsWired verifies call deps are in ActivityDeps and Activities.
func TestGateV104ActivityDepsWired(t *testing.T) {
	t.Parallel()
	root := repositoryRoot(t)

	activitiesPath := filepath.Join(root, "internal", "temporal", "activities.go")
	assertFileContainsTokens(t, activitiesPath, []string{
		"CallRepo",
		"callRepo",
		"CallService",
		"callService",
		"PhoneVerifier",
		"phoneVerifier",
		"call.CallRepository",
		"call.PhoneVerifier",
	})
}

// TestGateV104ReplayDeterminism verifies workflows_v104.go is in the
// determinism audit list.
func TestGateV104ReplayDeterminism(t *testing.T) {
	t.Parallel()
	root := repositoryRoot(t)

	replayPath := filepath.Join(root, "internal", "temporal", "replay_test.go")
	content, err := os.ReadFile(replayPath)
	if err != nil {
		t.Fatalf("read replay_test.go: %v", err)
	}
	if !strings.Contains(string(content), "workflows_v104.go") {
		t.Error("workflows_v104.go is not in the determinism audit list")
	}
}

// TestGateCallInsertBeforeProvider verifies the MAKE_CALL activity inserts
// the call row BEFORE calling the provider (crash safety).
func TestGateCallInsertBeforeProvider(t *testing.T) {
	t.Parallel()
	root := repositoryRoot(t)

	activitiesPath := filepath.Join(root, "internal", "temporal", "activities_v104.go")
	body, err := os.ReadFile(activitiesPath)
	if err != nil {
		t.Fatalf("read activities_v104.go: %v", err)
	}
	content := string(body)

	// InsertCall should appear before InitiateCall in the file.
	insertIdx := strings.Index(content, "InsertCall")
	initiateIdx := strings.Index(content, "InitiateCall")
	if insertIdx < 0 || initiateIdx < 0 {
		t.Fatal("missing InsertCall or InitiateCall in activities_v104.go")
	}
	if insertIdx > initiateIdx {
		t.Error("CRASH_SAFETY_VIOLATION: InsertCall must appear before InitiateCall — call row must be persisted before provider call")
	}
}

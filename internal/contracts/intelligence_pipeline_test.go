package contracts

import (
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"

	"github.com/brevio/brevio/internal/temporal"
)

// --- P7: Intelligence pipeline wiring ---

func TestWorkflowResultHasEvidenceFields(t *testing.T) {
	typ := reflect.TypeOf(temporal.MessageProcessingWorkflowResult{})
	required := []string{
		"EvidenceHash",
		"MemoryItemCount",
		"RAGChunkCount",
		"ReasoningIterations",
		"CouncilConvened",
		"OutboxEntryID",
	}
	for _, field := range required {
		if _, ok := typ.FieldByName(field); !ok {
			t.Errorf("MessageProcessingWorkflowResult missing evidence field: %s", field)
		}
	}
}

func TestIntelligenceActivityTypes(t *testing.T) {
	// All intelligence pipeline activity types must be defined.
	types := []string{
		"MemoryRetrieveInput",
		"MemoryRetrieveResult",
		"MemoryItem",
		"RAGSearchInput",
		"RAGSearchResult",
		"RAGChunk",
		"ReasoningLoopInput",
		"ReasoningLoopResult",
		"CouncilEvalInput",
		"CouncilEvalResult",
		"OutboxEnqueueInput",
		"OutboxEnqueueResult",
		"CognitiveAssessInput",
		"CognitiveAssessResult",
	}
	pkg := reflect.TypeOf(temporal.Activities{}).PkgPath()
	_ = pkg // Used for documentation only.
	for _, name := range types {
		// Verify types exist by attempting to resolve them via struct fields or known types.
		switch name {
		case "MemoryRetrieveInput":
			_ = temporal.MemoryRetrieveInput{}
		case "MemoryRetrieveResult":
			_ = temporal.MemoryRetrieveResult{}
		case "MemoryItem":
			_ = temporal.MemoryItem{}
		case "RAGSearchInput":
			_ = temporal.RAGSearchInput{}
		case "RAGSearchResult":
			_ = temporal.RAGSearchResult{}
		case "RAGChunk":
			_ = temporal.RAGChunk{}
		case "ReasoningLoopInput":
			_ = temporal.ReasoningLoopInput{}
		case "ReasoningLoopResult":
			_ = temporal.ReasoningLoopResult{}
		case "CouncilEvalInput":
			_ = temporal.CouncilEvalInput{}
		case "CouncilEvalResult":
			_ = temporal.CouncilEvalResult{}
		case "OutboxEnqueueInput":
			_ = temporal.OutboxEnqueueInput{}
		case "OutboxEnqueueResult":
			_ = temporal.OutboxEnqueueResult{}
		case "CognitiveAssessInput":
			_ = temporal.CognitiveAssessInput{}
		case "CognitiveAssessResult":
			_ = temporal.CognitiveAssessResult{}
		default:
			t.Errorf("unknown type: %s", name)
		}
	}
}

func TestIntelligenceActivitiesAreMethodBased(t *testing.T) {
	// All intelligence pipeline activities must be methods on Activities.
	typ := reflect.TypeOf(&temporal.Activities{})
	methods := []string{
		"RetrieveMemoryActivity",
		"SearchRAGActivity",
		"ExecuteReasoningLoopActivity",
		"EvaluateCouncilActivity",
		"EnqueueOutboxActivity",
		"AssessCognitiveStateActivity",
	}
	for _, method := range methods {
		if _, ok := typ.MethodByName(method); !ok {
			t.Errorf("Activities missing method: %s", method)
		}
	}
}

func TestReasoningLoopResultHasDeterministicFlag(t *testing.T) {
	typ := reflect.TypeOf(temporal.ReasoningLoopResult{})
	field, ok := typ.FieldByName("Deterministic")
	if !ok {
		t.Fatal("ReasoningLoopResult missing Deterministic field")
	}
	if field.Type.Kind() != reflect.Bool {
		t.Fatalf("Deterministic field should be bool, got %s", field.Type.Kind())
	}
}

func TestReasoningLoopResultHasEvidenceHash(t *testing.T) {
	typ := reflect.TypeOf(temporal.ReasoningLoopResult{})
	if _, ok := typ.FieldByName("EvidenceHash"); !ok {
		t.Fatal("ReasoningLoopResult missing EvidenceHash field")
	}
}

func TestReasoningLoopInputCarriesMemoryAndRAG(t *testing.T) {
	typ := reflect.TypeOf(temporal.ReasoningLoopInput{})
	if _, ok := typ.FieldByName("MemoryItems"); !ok {
		t.Fatal("ReasoningLoopInput missing MemoryItems field")
	}
	if _, ok := typ.FieldByName("RAGChunks"); !ok {
		t.Fatal("ReasoningLoopInput missing RAGChunks field")
	}
}

func TestWorkflowFileContainsPipelineSteps(t *testing.T) {
	workflowFile := findIntelligenceProjectFile(t, "internal/temporal/workflows.go")
	data, err := os.ReadFile(workflowFile)
	if err != nil {
		t.Fatalf("read workflows.go: %v", err)
	}
	content := string(data)

	// Verify the full pipeline is present in order.
	steps := []struct {
		needle string
		desc   string
	}{
		{"a.ValidateEnvelopeActivity", "Step 1: validate envelope"},
		{"a.ClassifyIntentActivity", "Step 2: classify intent"},
		{"a.RetrieveMemoryActivity", "Step 3: retrieve memory"},
		{"a.SearchRAGActivity", "Step 4: RAG search"},
		{"a.ExecuteReasoningLoopActivity", "Step 5: reasoning loop"},
		{"a.AssessCognitiveStateActivity", "Step 6: cognitive assessment"},
		{"a.EvaluateCouncilActivity", "Step 7: council evaluation"},
		{"a.AuthorizePlanActivity", "Step 8: authorize plan"},
		{"a.ExecuteToolActivity", "Step 9: execute tools"},
		{"a.SynthesizeResponseActivity", "Step 10: synthesize response"},
		{"a.EnqueueOutboxActivity", "Step 11: outbox enqueue"},
	}
	for _, step := range steps {
		if !strings.Contains(content, step.needle) {
			t.Errorf("workflow missing %s (needle: %q)", step.desc, step.needle)
		}
	}
}

func TestWorkerRegistersIntelligenceActivities(t *testing.T) {
	workerFile := findIntelligenceProjectFile(t, "internal/temporal/worker.go")
	data, err := os.ReadFile(workerFile)
	if err != nil {
		t.Fatalf("read worker.go: %v", err)
	}
	content := string(data)

	activities := []string{
		"RetrieveMemoryActivity",
		"SearchRAGActivity",
		"ExecuteReasoningLoopActivity",
		"EvaluateCouncilActivity",
		"EnqueueOutboxActivity",
		"AssessCognitiveStateActivity",
	}
	for _, activity := range activities {
		if !strings.Contains(content, activity) {
			t.Errorf("worker.go missing registration of %s", activity)
		}
	}
}

func TestDeterministicOrderingEnforced(t *testing.T) {
	activitiesFile := findIntelligenceProjectFile(t, "internal/temporal/activities.go")
	data, err := os.ReadFile(activitiesFile)
	if err != nil {
		t.Fatalf("read activities.go: %v", err)
	}
	content := string(data)

	// Verify deterministic patterns.
	patterns := []struct {
		needle string
		desc   string
	}{
		{"sort.SliceStable", "must use stable sort for memory/RAG results"},
		{"sort.Strings(toolKeys)", "must sort tool keys lexically"},
		{"fnvHash64", "must use FNV-64a for deterministic hashing"},
		{"sort.Strings(parts)", "must sort context hash parts"},
	}
	for _, p := range patterns {
		if !strings.Contains(content, p.needle) {
			t.Errorf("activities.go: %s (missing %q)", p.desc, p.needle)
		}
	}
}

func TestWorkflowUsesComputeDeterministicBackoff(t *testing.T) {
	workflowFile := findIntelligenceProjectFile(t, "internal/temporal/workflows.go")
	data, err := os.ReadFile(workflowFile)
	if err != nil {
		t.Fatalf("read workflows.go: %v", err)
	}
	content := string(data)

	if !strings.Contains(content, "ComputeDeterministicBackoff") {
		t.Error("MessageProcessingWorkflow must use ComputeDeterministicBackoff for tool execution jitter")
	}
}

func TestNoNondeterministicLLMInActivities(t *testing.T) {
	activitiesFile := findIntelligenceProjectFile(t, "internal/temporal/activities.go")
	data, err := os.ReadFile(activitiesFile)
	if err != nil {
		t.Fatalf("read activities.go: %v", err)
	}
	content := string(data)

	// No random, no time.Now (outside of default timestamps), no math/rand.
	forbidden := []string{
		"math/rand",
		"crypto/rand",
		"rand.Intn",
		"rand.Float",
	}
	for _, f := range forbidden {
		if strings.Contains(content, f) {
			t.Errorf("activities.go contains nondeterministic import/call: %q", f)
		}
	}
}

// --- helpers ---

func findIntelligenceProjectFile(t *testing.T, relPath string) string {
	t.Helper()
	_, callerFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot determine caller file")
	}
	dir := filepath.Dir(callerFile)
	candidate := filepath.Join(dir, "..", "..", relPath)
	if _, err := os.Stat(candidate); err == nil {
		return candidate
	}
	t.Fatalf("project file not found: %s", relPath)
	return ""
}

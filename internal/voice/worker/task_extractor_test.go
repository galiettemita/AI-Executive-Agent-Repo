package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/brevio/brevio/internal/llm"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockLLM implements llm.Client for testing.
type mockLLM struct {
	content string
	err     error
}

func (m *mockLLM) Generate(_ context.Context, _ llm.GenerateRequest) (*llm.GenerateResponse, *llm.Usage, error) {
	if m.err != nil {
		return nil, nil, m.err
	}
	return &llm.GenerateResponse{Content: m.content}, &llm.Usage{}, nil
}

func (m *mockLLM) Stream(_ context.Context, _ llm.GenerateRequest, out chan<- llm.StreamChunk) {
	defer close(out)
	out <- llm.StreamChunk{Delta: m.content}
	out <- llm.StreamChunk{Done: true}
}

var goodTasksJSON = `{
  "tasks": [
    {"description":"Send Q3 forecast","assignee":"Bob","due_date":"2026-04-15","priority":"high","speaker":"Speaker 1","confidence":0.95},
    {"description":"Schedule standup","assignee":"","due_date":"","priority":"medium","speaker":"Speaker 2","confidence":0.88}
  ],
  "summary":"Team discussed Q3 deliverables."
}`

func TestNewLLMTaskExtractor_NilClient(t *testing.T) {
	_, err := NewLLMTaskExtractor(LLMTaskExtractorConfig{})
	require.Error(t, err)
}

func TestLLMTaskExtractor_EmptyTranscript(t *testing.T) {
	e, _ := NewLLMTaskExtractor(LLMTaskExtractorConfig{LLMClient: &mockLLM{}})
	r, err := e.Extract(context.Background(), "")
	require.NoError(t, err)
	assert.Empty(t, r.Tasks)
}

func TestLLMTaskExtractor_HappyPath(t *testing.T) {
	e, _ := NewLLMTaskExtractor(LLMTaskExtractorConfig{
		LLMClient: &mockLLM{content: goodTasksJSON}, MinConfidence: 0.5,
	})
	r, err := e.Extract(context.Background(), "some transcript")
	require.NoError(t, err)
	assert.Len(t, r.Tasks, 2)
	assert.Equal(t, "2026-04-15", r.Tasks[0].DueDate)
	assert.Contains(t, []string{"high", "medium", "low"}, r.Tasks[0].Priority)
}

func TestLLMTaskExtractor_LowConfidenceFiltered(t *testing.T) {
	j := `{"tasks":[{"description":"do stuff","assignee":"","due_date":"","priority":"low","speaker":"S1","confidence":0.3}],"summary":""}`
	e, _ := NewLLMTaskExtractor(LLMTaskExtractorConfig{LLMClient: &mockLLM{content: j}, MinConfidence: 0.65})
	r, err := e.Extract(context.Background(), "transcript")
	require.NoError(t, err)
	assert.Empty(t, r.Tasks)
}

func TestLLMTaskExtractor_MalformedJSON_FallbackEnabled(t *testing.T) {
	e, _ := NewLLMTaskExtractor(LLMTaskExtractorConfig{
		LLMClient: &mockLLM{content: "not json"}, FallbackOnFail: true,
	})
	r, err := e.Extract(context.Background(), "remind me to call John")
	require.NoError(t, err)
	assert.NotNil(t, r)
}

func TestLLMTaskExtractor_MalformedJSON_FallbackDisabled(t *testing.T) {
	e, _ := NewLLMTaskExtractor(LLMTaskExtractorConfig{
		LLMClient: &mockLLM{content: "not json"}, FallbackOnFail: false,
	})
	_, err := e.Extract(context.Background(), "test transcript")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unmarshal")
}

func TestLLMTaskExtractor_LLMError_FallbackEnabled(t *testing.T) {
	e, _ := NewLLMTaskExtractor(LLMTaskExtractorConfig{
		LLMClient: &mockLLM{err: fmt.Errorf("rate_limit")}, FallbackOnFail: true,
	})
	r, err := e.Extract(context.Background(), "schedule meeting tomorrow")
	require.NoError(t, err)
	assert.NotNil(t, r)
}

func TestLLMTaskExtractor_LLMError_FallbackDisabled(t *testing.T) {
	e, _ := NewLLMTaskExtractor(LLMTaskExtractorConfig{
		LLMClient: &mockLLM{err: fmt.Errorf("rate_limit")}, FallbackOnFail: false,
	})
	_, err := e.Extract(context.Background(), "test")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "LLM call failed")
}

func TestLLMTaskExtractor_DateNormalisation_Valid(t *testing.T) {
	j := `{"tasks":[{"description":"task","assignee":"","due_date":"2026-04-15","priority":"low","speaker":"","confidence":0.9}],"summary":""}`
	e, _ := NewLLMTaskExtractor(LLMTaskExtractorConfig{LLMClient: &mockLLM{content: j}, MinConfidence: 0.5})
	r, _ := e.Extract(context.Background(), "x")
	assert.Equal(t, "2026-04-15", r.Tasks[0].DueDate)
}

func TestLLMTaskExtractor_DateNormalisation_Invalid(t *testing.T) {
	j := `{"tasks":[{"description":"task","assignee":"","due_date":"next Friday","priority":"low","speaker":"","confidence":0.9}],"summary":""}`
	e, _ := NewLLMTaskExtractor(LLMTaskExtractorConfig{LLMClient: &mockLLM{content: j}, MinConfidence: 0.5})
	r, _ := e.Extract(context.Background(), "x")
	assert.Equal(t, "", r.Tasks[0].DueDate)
}

func TestLLMTaskExtractor_MaxTasksClamped(t *testing.T) {
	tasks := make([]map[string]interface{}, 25)
	for i := range tasks {
		tasks[i] = map[string]interface{}{
			"description": "task", "assignee": "", "due_date": "",
			"priority": "low", "speaker": "", "confidence": 0.9,
		}
	}
	b, _ := json.Marshal(map[string]interface{}{"tasks": tasks, "summary": ""})
	e, _ := NewLLMTaskExtractor(LLMTaskExtractorConfig{
		LLMClient: &mockLLM{content: string(b)}, MaxTasks: 5, MinConfidence: 0.5,
	})
	r, _ := e.Extract(context.Background(), "x")
	assert.Len(t, r.Tasks, 5)
}

func TestLLMTaskExtractor_MarkdownFencesStripped(t *testing.T) {
	wrapped := "```json\n" + goodTasksJSON + "\n```"
	e, _ := NewLLMTaskExtractor(LLMTaskExtractorConfig{LLMClient: &mockLLM{content: wrapped}, MinConfidence: 0.5})
	r, err := e.Extract(context.Background(), "test")
	require.NoError(t, err)
	assert.Len(t, r.Tasks, 2)
}

func TestLLMTaskExtractor_ExtractFromTurns(t *testing.T) {
	e, _ := NewLLMTaskExtractor(LLMTaskExtractorConfig{LLMClient: &mockLLM{content: goodTasksJSON}, MinConfidence: 0.5})
	turns := []TranscriptTurn{{Speaker: "S1", Text: "send report"}, {Speaker: "S2", Text: "schedule meeting"}}
	result, err := e.ExtractFromTurns(context.Background(), turns)
	require.NoError(t, err)
	assert.IsType(t, []ExtractedTask{}, result)
}

func TestKeywordTaskExtractor_PatternMatch(t *testing.T) {
	k := NewKeywordTaskExtractor()
	turns := []TranscriptTurn{{Speaker: "Alice", Text: "remind me to call John tomorrow"}}
	tasks := k.ExtractTasks(turns)
	assert.Len(t, tasks, 1)
	assert.Contains(t, tasks[0].Description, "call John")
}

func TestKeywordTaskExtractor_FalsePositiveDocumented(t *testing.T) {
	// KNOWN LIMITATION: keyword extractor matches "please" in filler text.
	k := NewKeywordTaskExtractor()
	turns := []TranscriptTurn{{Speaker: "Alice", Text: "please hold for a moment"}}
	tasks := k.ExtractTasks(turns)
	assert.GreaterOrEqual(t, len(tasks), 1, "keyword extractor (falsely) matches 'please'")
}

func TestSanitiseTask_PriorityNormalisation(t *testing.T) {
	assert.Equal(t, "high", sanitiseTask(StructuredTask{Priority: "HIGH", Confidence: 0.9}).Priority)
	assert.Equal(t, "low", sanitiseTask(StructuredTask{Priority: "URGENT", Confidence: 0.9}).Priority)
}

func TestTurnsToTranscript_Format(t *testing.T) {
	turns := []TranscriptTurn{{Speaker: "Speaker A", Text: "hello"}, {Speaker: "Speaker B", Text: "world"}}
	result := turnsToTranscript(turns)
	assert.Contains(t, result, "Speaker A: hello\n")
	assert.Contains(t, result, "Speaker B: world\n")
}

func TestExtractedTaskToStructuredTask_Priority(t *testing.T) {
	e := ExtractedTask{Description: "do thing", AssignedTo: "Bob", Priority: 1}
	s := e.ToStructuredTask()
	assert.Equal(t, "high", s.Priority)
	assert.Equal(t, 0.5, s.Confidence)
}

func TestPriorityStringToInt(t *testing.T) {
	assert.Equal(t, 1, priorityStringToInt("high"))
	assert.Equal(t, 2, priorityStringToInt("medium"))
	assert.Equal(t, 3, priorityStringToInt("low"))
	assert.Equal(t, 3, priorityStringToInt("unknown"))
}

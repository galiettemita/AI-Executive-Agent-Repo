package kg

import (
	"context"
	"fmt"
	"testing"
)

type mockExtractorLLM struct {
	response string
	err      error
}

func (m *mockExtractorLLM) Complete(_ context.Context, _, _ string) (string, error) {
	return m.response, m.err
}

type nopKGLogger struct{}

func (nopKGLogger) Info(string, ...any)  {}
func (nopKGLogger) Warn(string, ...any)  {}
func (nopKGLogger) Error(string, ...any) {}
func (nopKGLogger) Debug(string, ...any) {}

func TestExtractor_ExtractsPersonRelation(t *testing.T) {
	llm := &mockExtractorLLM{response: `[{"subject":"Alice Chen","predicate":"reports_to","object":"Bob Smith","subject_type":"person","object_type":"person","confidence":0.95}]`}
	ext := NewLLMExtractor(llm, nopKGLogger{})

	triples, err := ext.Extract(context.Background(), ExtractionRequest{
		WorkspaceID: "ws-1", TurnID: "t1",
		Content: "Alice Chen reports to Bob Smith in the engineering org",
		Role:    "user",
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(triples) != 1 {
		t.Fatalf("expected 1 triple, got %d", len(triples))
	}
	if triples[0].Subject != "Alice Chen" || triples[0].Object != "Bob Smith" {
		t.Fatalf("unexpected triple: %+v", triples[0])
	}
}

func TestExtractor_RejectsLowConfidence(t *testing.T) {
	llm := &mockExtractorLLM{response: `[{"subject":"A","predicate":"knows","object":"B","subject_type":"person","object_type":"person","confidence":0.5}]`}
	ext := NewLLMExtractor(llm, nopKGLogger{})

	triples, _ := ext.Extract(context.Background(), ExtractionRequest{
		WorkspaceID: "ws-1", TurnID: "t1",
		Content: "A might know B but we are not sure about this",
		Role:    "user",
	})
	if len(triples) != 0 {
		t.Fatalf("expected 0 triples (low confidence), got %d", len(triples))
	}
}

func TestExtractor_SelfReference_Filtered(t *testing.T) {
	llm := &mockExtractorLLM{response: `[{"subject":"Alice","predicate":"manages","object":"Alice","subject_type":"person","object_type":"person","confidence":0.95}]`}
	ext := NewLLMExtractor(llm, nopKGLogger{})

	triples, _ := ext.Extract(context.Background(), ExtractionRequest{
		WorkspaceID: "ws-1", TurnID: "t1",
		Content: "Alice manages Alice in the system according to the records",
		Role:    "user",
	})
	if len(triples) != 0 {
		t.Fatal("self-referential triple should be filtered")
	}
}

func TestExtractor_ShortText_SkipsLLM(t *testing.T) {
	called := false
	llm := &mockExtractorLLM{response: "[]"}
	_ = llm
	trackLLM := &trackingLLM{called: &called}
	ext := NewLLMExtractor(trackLLM, nopKGLogger{})

	triples, _ := ext.Extract(context.Background(), ExtractionRequest{
		WorkspaceID: "ws-1", TurnID: "t1",
		Content: "ok thanks",
		Role:    "user",
	})
	if called {
		t.Error("LLM should not be called for short text")
	}
	if triples != nil {
		t.Error("expected nil for short text")
	}
}

func TestExtractor_MalformedJSON_NilResult(t *testing.T) {
	llm := &mockExtractorLLM{response: "I cannot help with that."}
	ext := NewLLMExtractor(llm, nopKGLogger{})

	triples, err := ext.Extract(context.Background(), ExtractionRequest{
		WorkspaceID: "ws-1", TurnID: "t1",
		Content: "Alice Chen reports to Bob Smith in the engineering org",
		Role:    "user",
	})
	if err != nil {
		t.Fatal("expected nil error")
	}
	if triples != nil {
		t.Fatal("expected nil triples on malformed JSON")
	}
}

func TestExtractor_NormalizesPredicate(t *testing.T) {
	llm := &mockExtractorLLM{response: `[{"subject":"Alice","predicate":"reports to","object":"Bob","subject_type":"person","object_type":"person","confidence":0.9}]`}
	ext := NewLLMExtractor(llm, nopKGLogger{})

	triples, _ := ext.Extract(context.Background(), ExtractionRequest{
		WorkspaceID: "ws-1", TurnID: "t1",
		Content: "Alice reports to Bob in the engineering organization",
		Role:    "user",
	})
	if len(triples) != 1 {
		t.Fatalf("expected 1 triple, got %d", len(triples))
	}
	if triples[0].Predicate != "reports_to" {
		t.Fatalf("predicate not normalized: got %q", triples[0].Predicate)
	}
}

func TestExtractor_LLMError_NilResult(t *testing.T) {
	llm := &mockExtractorLLM{err: fmt.Errorf("HTTP error")}
	ext := NewLLMExtractor(llm, nopKGLogger{})

	triples, err := ext.Extract(context.Background(), ExtractionRequest{
		WorkspaceID: "ws-1", TurnID: "t1",
		Content: "Alice Chen reports to Bob Smith in the engineering org",
		Role:    "user",
	})
	if err != nil {
		t.Fatal("expected nil error on LLM failure")
	}
	if triples != nil {
		t.Fatal("expected nil triples on LLM failure")
	}
}

type trackingLLM struct {
	called *bool
}

func (m *trackingLLM) Complete(_ context.Context, _, _ string) (string, error) {
	*m.called = true
	return "[]", nil
}

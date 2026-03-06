package workflows

import "strings"

type MemoryOperationState string

const (
	MemoryStateInit       MemoryOperationState = "INIT"
	MemoryStateChunking   MemoryOperationState = "CHUNKING"
	MemoryStateEmbedding  MemoryOperationState = "EMBEDDING"
	MemoryStateIndexing   MemoryOperationState = "INDEXING"
	MemoryStateSearching  MemoryOperationState = "SEARCHING"
	MemoryStateRanking    MemoryOperationState = "RANKING"
	MemoryStateCompleted  MemoryOperationState = "COMPLETED"
	MemoryStateFailed     MemoryOperationState = "FAILED"
)

type MemoryStoreWorkflowInput struct {
	DocumentID    string
	UserID        string
	ContentLength int
	ChunkError    bool
	EmbedError    bool
	IndexError    bool
}

type MemoryStoreWorkflowResult struct {
	WorkflowID    string
	States        []MemoryOperationState
	TerminalState MemoryOperationState
	Fallbacks     []string
	ChunkCount    int
}

type MemoryRecallWorkflowInput struct {
	QueryID       string
	UserID        string
	SearchError   bool
	RankError     bool
	ResultCount   int
}

type MemoryRecallWorkflowResult struct {
	WorkflowID    string
	States        []MemoryOperationState
	TerminalState MemoryOperationState
	Fallbacks     []string
}

func MemoryStoreWorkflowID(documentID string) string {
	return "mem-store-" + strings.TrimSpace(documentID)
}

func MemoryRecallWorkflowID(queryID string) string {
	return "mem-recall-" + strings.TrimSpace(queryID)
}

func (s *Service) MemoryStoreWorkflowV1(input MemoryStoreWorkflowInput) MemoryStoreWorkflowResult {
	workflowID := MemoryStoreWorkflowID(input.DocumentID)
	result := MemoryStoreWorkflowResult{
		WorkflowID: workflowID,
		States:     []MemoryOperationState{MemoryStateInit},
		Fallbacks:  []string{},
	}

	result.States = append(result.States, MemoryStateChunking)
	if input.ChunkError {
		result.Fallbacks = append(result.Fallbacks, "single_chunk")
	}
	result.ChunkCount = maxInt(1, input.ContentLength/512)

	result.States = append(result.States, MemoryStateEmbedding)
	if input.EmbedError {
		result.States = append(result.States, MemoryStateFailed)
		result.TerminalState = MemoryStateFailed
		return result
	}

	result.States = append(result.States, MemoryStateIndexing)
	if input.IndexError {
		result.Fallbacks = append(result.Fallbacks, "retry_index")
	}

	result.States = append(result.States, MemoryStateCompleted)
	result.TerminalState = MemoryStateCompleted
	return result
}

func (s *Service) MemoryRecallWorkflowV1(input MemoryRecallWorkflowInput) MemoryRecallWorkflowResult {
	workflowID := MemoryRecallWorkflowID(input.QueryID)
	result := MemoryRecallWorkflowResult{
		WorkflowID: workflowID,
		States:     []MemoryOperationState{MemoryStateInit},
		Fallbacks:  []string{},
	}

	result.States = append(result.States, MemoryStateSearching)
	if input.SearchError {
		result.Fallbacks = append(result.Fallbacks, "keyword_search")
	}

	result.States = append(result.States, MemoryStateRanking)
	if input.RankError {
		result.Fallbacks = append(result.Fallbacks, "recency_rank")
	}

	result.States = append(result.States, MemoryStateCompleted)
	result.TerminalState = MemoryStateCompleted
	return result
}


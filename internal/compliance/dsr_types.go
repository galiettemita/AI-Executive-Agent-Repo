package compliance

// DSRDeletedCounts tracks per-store deletion counts for compliance evidence.
type DSRDeletedCounts struct {
	EpisodicMemory int `json:"episodic_memory"`
	KGTriples      int `json:"kg_triples"`
	VectorChunks   int `json:"vector_chunks"`
	ExecutionLogs  int `json:"execution_logs"`
	PIIFields      int `json:"pii_fields"`
	ConsentRecords int `json:"consent_records"`
}

package adaptive

// RetrievalTier classifies a query's retrieval requirements.
type RetrievalTier int

const (
	TierNoRetrieval RetrievalTier = iota
	TierSingleHop
	TierMultiHop
)

func (t RetrievalTier) String() string {
	switch t {
	case TierNoRetrieval:
		return "no_retrieval"
	case TierSingleHop:
		return "single_hop"
	case TierMultiHop:
		return "multi_hop"
	default:
		return "unknown"
	}
}

// ClassificationResult is the output of the RetrievalClassifier.
type ClassificationResult struct {
	Tier       RetrievalTier
	Confidence float64
	Method     string // "rule_based" or "llm" or "fallback"
	Reason     string
}

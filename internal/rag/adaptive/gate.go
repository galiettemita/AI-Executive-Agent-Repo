package adaptive

import (
	"context"
	"time"
)

// GateMetrics records per-tier counters and latencies.
type GateMetrics interface {
	IncQueryClassified(tier, method string)
	ObserveClassificationLatency(tier string, duration time.Duration)
	IncRetrievalSkipped()
}

// NoopGateMetrics is a no-op implementation for use in tests.
type NoopGateMetrics struct{}

func (n *NoopGateMetrics) IncQueryClassified(_, _ string)                          {}
func (n *NoopGateMetrics) ObserveClassificationLatency(_ string, _ time.Duration)  {}
func (n *NoopGateMetrics) IncRetrievalSkipped()                                     {}

// Gate routes RAG requests through the appropriate pipeline based on query tier.
type Gate struct {
	classifier *RetrievalClassifier
	metrics    GateMetrics
	logger     Logger
}

func NewGate(classifier *RetrievalClassifier, metrics GateMetrics, logger Logger) *Gate {
	if metrics == nil {
		metrics = &NoopGateMetrics{}
	}
	return &Gate{
		classifier: classifier,
		metrics:    metrics,
		logger:     logger,
	}
}

// Route classifies the query and returns the routing decision.
func (g *Gate) Route(ctx context.Context, query string) ClassificationResult {
	start := time.Now()
	result := g.classifier.Classify(ctx, query)
	elapsed := time.Since(start)

	g.metrics.IncQueryClassified(result.Tier.String(), result.Method)
	g.metrics.ObserveClassificationLatency(result.Tier.String(), elapsed)

	if result.Tier == TierNoRetrieval {
		g.metrics.IncRetrievalSkipped()
	}

	g.logger.Debug("adaptive_rag: classified",
		"tier", result.Tier.String(),
		"confidence", result.Confidence,
		"method", result.Method,
		"reason", result.Reason,
		"latency_ms", elapsed.Milliseconds(),
	)

	return result
}

// ShouldSkipRetrieval returns true when retrieval should be bypassed entirely.
func (g *Gate) ShouldSkipRetrieval(ctx context.Context, query string) bool {
	result := g.Route(ctx, query)
	return result.Tier == TierNoRetrieval
}

// IsMultiHop returns true when the query needs cross-collection retrieval.
func (g *Gate) IsMultiHop(ctx context.Context, query string) bool {
	result := g.Route(ctx, query)
	return result.Tier == TierMultiHop
}

// Prometheus metric names (register with your existing Prometheus registry):
//
//   adaptive_rag_queries_classified_total{tier="no_retrieval|single_hop|multi_hop", method="rule_based|llm|fallback"}
//   adaptive_rag_classification_duration_seconds{tier="no_retrieval|single_hop|multi_hop"}
//   adaptive_rag_retrieval_skipped_total

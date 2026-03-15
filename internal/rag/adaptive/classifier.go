package adaptive

import (
	"context"
	"math"
	"strings"
	"unicode"
)

// ClassifierLLMClient is the minimal interface for LLM-based classification.
type ClassifierLLMClient interface {
	Complete(ctx context.Context, system, user string) (string, error)
}

// Logger is the minimal logging interface.
type Logger interface {
	Debug(msg string, args ...any)
	Warn(msg string, args ...any)
	Error(msg string, args ...any)
}

// RetrievalClassifier classifies queries into retrieval tiers.
type RetrievalClassifier struct {
	llm                ClassifierLLMClient
	logger             Logger
	ambiguityThreshold float64
}

func NewRetrievalClassifier(llm ClassifierLLMClient, logger Logger) *RetrievalClassifier {
	return &RetrievalClassifier{
		llm:                llm,
		logger:             logger,
		ambiguityThreshold: 0.75,
	}
}

// Classify determines the retrieval tier for a query.
func (c *RetrievalClassifier) Classify(ctx context.Context, query string) ClassificationResult {
	query = strings.TrimSpace(query)
	if query == "" {
		return ClassificationResult{
			Tier: TierNoRetrieval, Confidence: 1.0,
			Method: "rule_based", Reason: "empty query",
		}
	}

	result := c.classifyByRules(query)
	if result.Confidence >= c.ambiguityThreshold {
		return result
	}

	if c.llm != nil {
		llmResult := c.classifyByLLM(ctx, query)
		if llmResult.Confidence > 0 {
			return llmResult
		}
	}

	return ClassificationResult{
		Tier: TierSingleHop, Confidence: 0.5,
		Method: "fallback", Reason: "ambiguous query; defaulting to single-hop retrieval",
	}
}

// noRetrievalPatterns are exact matches for turns that need no retrieval.
var noRetrievalPatterns = []string{
	"ok", "okay", "got it", "thanks", "thank you", "sure", "sounds good",
	"perfect", "great", "understood", "noted", "will do", "yep", "yes",
	"no", "nope", "correct", "exactly", "right", "agreed", "makes sense",
	"yes please", "no thanks", "good idea", "works for me", "of course",
	"absolutely", "definitely", "sure thing", "on it", "done",
}

var multiHopIndicators = []string{
	"everything", "all ", "summarize", "summary", "across", "over the past",
	"this month", "this week", "this quarter", "throughout", "in total",
	"all the times", "whenever", "every time", "all meetings", "all emails",
	"all documents", "connected to", "related to", "associated with",
	"everyone who", "all people", "all projects",
}

var multiHopQuestionStarters = []string{
	"who are all", "what are all", "list all", "show me all",
	"give me everything", "find everything", "compile", "aggregate",
}

func (c *RetrievalClassifier) classifyByRules(query string) ClassificationResult {
	lower := strings.ToLower(query)
	wordCount := len(strings.Fields(query))

	// NO_RETRIEVAL: very short non-question
	if wordCount <= 3 && !isQuestion(lower) {
		return ClassificationResult{
			Tier: TierNoRetrieval, Confidence: 0.90,
			Method: "rule_based", Reason: "very short non-question turn",
		}
	}

	// NO_RETRIEVAL: exact acknowledgement pattern
	for _, pat := range noRetrievalPatterns {
		if strings.EqualFold(strings.TrimRight(query, ".,!?"), pat) {
			return ClassificationResult{
				Tier: TierNoRetrieval, Confidence: 0.98,
				Method: "rule_based",
				Reason: "matched acknowledgement pattern: " + pat,
			}
		}
	}

	// NO_RETRIEVAL: short conversational continuation
	if wordCount <= 6 && (strings.HasPrefix(lower, "and ") ||
		strings.HasPrefix(lower, "but ") || strings.HasPrefix(lower, "also ")) &&
		!isQuestion(lower) {
		return ClassificationResult{
			Tier: TierNoRetrieval, Confidence: 0.82,
			Method: "rule_based", Reason: "short conversational continuation",
		}
	}

	// MULTI_HOP: aggregation language
	multiHopScore := 0
	for _, indicator := range multiHopIndicators {
		if strings.Contains(lower, indicator) {
			multiHopScore++
		}
	}
	for _, starter := range multiHopQuestionStarters {
		if strings.HasPrefix(lower, starter) {
			multiHopScore += 2
		}
	}
	if multiHopScore >= 2 {
		return ClassificationResult{
			Tier: TierMultiHop, Confidence: math.Min(0.95, 0.75+float64(multiHopScore)*0.05),
			Method: "rule_based", Reason: "aggregation/cross-collection signals detected",
		}
	}

	// MULTI_HOP: long query with multiple entities
	if wordCount >= 15 && countProperNouns(query) >= 3 {
		return ClassificationResult{
			Tier: TierMultiHop, Confidence: 0.78,
			Method: "rule_based", Reason: "long query with multiple entities",
		}
	}

	// SINGLE_HOP: targeted question
	if isQuestion(lower) && wordCount >= 4 && wordCount <= 20 && multiHopScore == 0 {
		return ClassificationResult{
			Tier: TierSingleHop, Confidence: 0.85,
			Method: "rule_based", Reason: "targeted question, moderate length",
		}
	}

	// SINGLE_HOP: single-entity task request
	if isTaskRequest(lower) && countProperNouns(query) == 1 {
		return ClassificationResult{
			Tier: TierSingleHop, Confidence: 0.80,
			Method: "rule_based", Reason: "single-entity task request",
		}
	}

	// Ambiguous — let LLM decide
	return ClassificationResult{
		Tier: TierSingleHop, Confidence: 0.60,
		Method: "rule_based", Reason: "ambiguous; confidence below LLM threshold",
	}
}

const classificationSystemPrompt = `You are a query classifier for an AI executive assistant retrieval system.

Classify the user's message into ONE of three retrieval tiers:

NO_RETRIEVAL — conversational acknowledgement, confirmation, or simple direct response.
SINGLE_HOP — asks about a specific topic, person, document, or event.
MULTI_HOP — requires searching multiple collections, aggregation, or entity traversal.

OUTPUT FORMAT — respond with ONLY one of: NO_RETRIEVAL, SINGLE_HOP, MULTI_HOP`

func (c *RetrievalClassifier) classifyByLLM(ctx context.Context, query string) ClassificationResult {
	raw, err := c.llm.Complete(ctx, classificationSystemPrompt,
		"Classify this message: "+query)
	if err != nil {
		reason := "LLM classification failed"
		if strings.Contains(err.Error(), "circuit open") {
			reason = "LLM circuit open; defaulting to single-hop"
		} else if strings.Contains(err.Error(), "deadline exceeded") || strings.Contains(err.Error(), "llm_timeout") {
			reason = "LLM classification timed out; defaulting to single-hop"
		}
		c.logger.Warn("adaptive_rag: "+reason,
			"query_prefix", safePrefix(query, 60), "error", err)
		return ClassificationResult{
			Tier: TierSingleHop, Confidence: 0.50,
			Method: "fallback", Reason: reason,
		}
	}

	raw = strings.TrimSpace(raw)
	switch raw {
	case "NO_RETRIEVAL":
		return ClassificationResult{
			Tier: TierNoRetrieval, Confidence: 0.88,
			Method: "llm", Reason: "LLM classified as no-retrieval",
		}
	case "SINGLE_HOP":
		return ClassificationResult{
			Tier: TierSingleHop, Confidence: 0.88,
			Method: "llm", Reason: "LLM classified as single-hop",
		}
	case "MULTI_HOP":
		return ClassificationResult{
			Tier: TierMultiHop, Confidence: 0.88,
			Method: "llm", Reason: "LLM classified as multi-hop",
		}
	default:
		c.logger.Warn("adaptive_rag: unexpected LLM output", "output", raw)
		return ClassificationResult{}
	}
}

func isQuestion(lower string) bool {
	questionWords := []string{"what", "when", "where", "who", "how", "why",
		"which", "can you", "could you", "would you", "did", "does", "is ",
		"are ", "was ", "were ", "have ", "has ", "do you", "find me"}
	for _, qw := range questionWords {
		if strings.HasPrefix(lower, qw) {
			return true
		}
	}
	return strings.HasSuffix(strings.TrimRight(lower, " "), "?")
}

func isTaskRequest(lower string) bool {
	actionVerbs := []string{
		"schedule", "book", "send", "draft", "create", "find", "get",
		"cancel", "reschedule", "update", "add", "remove", "forward",
		"remind", "check", "pull", "fetch", "look up",
	}
	for _, v := range actionVerbs {
		if strings.HasPrefix(lower, v+" ") {
			return true
		}
	}
	return false
}

func countProperNouns(text string) int {
	words := strings.Fields(text)
	count := 0
	for i, word := range words {
		cleaned := strings.Trim(word, ".,!?;:()")
		if len(cleaned) == 0 || i == 0 {
			continue
		}
		if len(cleaned) > 1 && unicode.IsUpper(rune(cleaned[0])) {
			count++
		}
	}
	return count
}

func safePrefix(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}

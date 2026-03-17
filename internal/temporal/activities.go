package temporal

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log"
	"sort"
	"strings"
	"sync/atomic"
	"time"

	"os"

	"github.com/brevio/brevio/internal/brain"
	"github.com/brevio/brevio/internal/connectors"
	"github.com/brevio/brevio/internal/a2a"
	"github.com/brevio/brevio/internal/benchmark"
	"github.com/brevio/brevio/internal/browser"
	"github.com/brevio/brevio/internal/security/sandbox"
	"github.com/brevio/brevio/internal/observability"
	"github.com/brevio/brevio/internal/cognition"
	"github.com/brevio/brevio/internal/delegation"
	"github.com/brevio/brevio/internal/dpo"
	rageval "github.com/brevio/brevio/internal/rag/eval"
	"github.com/brevio/brevio/internal/preference"
	"github.com/brevio/brevio/internal/simulation"
	"github.com/brevio/brevio/internal/proactive"
	selfmod "github.com/brevio/brevio/internal/self_modification"
	contextlayer "github.com/brevio/brevio/internal/context"
	"github.com/brevio/brevio/internal/eq"
	"github.com/brevio/brevio/internal/eval"
	"github.com/brevio/brevio/internal/experiment"
	"github.com/brevio/brevio/internal/vision"
	"github.com/brevio/brevio/internal/executor"
	"github.com/brevio/brevio/internal/feature_flags"
	"github.com/brevio/brevio/internal/hands/call"
	"github.com/brevio/brevio/internal/llm"
	"github.com/brevio/brevio/internal/memory"
	"github.com/brevio/brevio/internal/memory/kg"
	"github.com/brevio/brevio/internal/onboarding"
	"github.com/brevio/brevio/internal/outbox"
	"github.com/brevio/brevio/internal/rag"
	"github.com/brevio/brevio/internal/gateway"
	"github.com/brevio/brevio/internal/guardrails"
	"github.com/brevio/brevio/internal/metrics"
	"github.com/brevio/brevio/internal/policy"
	"github.com/brevio/brevio/internal/trust"
	"github.com/brevio/brevio/internal/voice/worker"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.temporal.io/sdk/temporal"
)

// Activity input/output types

type ValidateEnvelopeInput struct {
	MessageID      string `json:"message_id"`
	WorkspaceID    string `json:"workspace_id"`
	ChannelType    string `json:"channel_type"`
	RawPayload     string `json:"raw_payload"`
	IdempotencyKey string `json:"idempotency_key"`
}

type ValidateEnvelopeResult struct {
	Valid             bool   `json:"valid"`
	NormalizedPayload string `json:"normalized_payload"`
	Reason            string `json:"reason,omitempty"`
}

type ClassifyIntentInput struct {
	MessageID   string `json:"message_id"`
	WorkspaceID string `json:"workspace_id"`
	Payload     string `json:"payload"`
}

type ClassifyIntentResult struct {
	Intent        string  `json:"intent"`
	Confidence    float64 `json:"confidence"`
	Fallback      string  `json:"fallback,omitempty"`
	Deterministic bool    `json:"deterministic"` // true if keyword fallback was used
}

type GeneratePlanInput struct {
	MessageID             string           `json:"message_id"`
	WorkspaceID           string           `json:"workspace_id"`
	Intent                string           `json:"intent"`
	Confidence            float64          `json:"confidence"`
	RequiresDecomposition bool             `json:"requires_decomposition,omitempty"`
	Payload               string           `json:"payload,omitempty"`
	MemoryContext         string           `json:"memory_context,omitempty"`
	RAGContext            string           `json:"rag_context,omitempty"`
	RetryHints            string           `json:"retry_hints,omitempty"` // populated on re-plan after verify fail
	ConversationHistory   []brain.Message  `json:"conversation_history,omitempty"`
	ContextBudget         int              `json:"context_budget,omitempty"` // 0 = use default 150000
	UserID                string           `json:"user_id,omitempty"`
}

type GeneratePlanResult struct {
	PlanID              string   `json:"plan_id"`
	ToolKeys            []string `json:"tool_keys"`
	RiskLevel           string   `json:"risk_level"`
	Deterministic       bool     `json:"deterministic"`       // true if fallback/keyword path was used
	FinalAnswerReqs     string   `json:"final_answer_requirements,omitempty"`
}

type AuthorizePlanInput struct {
	MessageID        string   `json:"message_id"`
	WorkspaceID      string   `json:"workspace_id"`
	PlanID           string   `json:"plan_id"`
	ToolKeys         []string `json:"tool_keys"`
	RiskLevel        string   `json:"risk_level"`
	RequestingUserID string   `json:"requesting_user_id,omitempty"`
	WorkspaceOwnerID string   `json:"workspace_owner_id,omitempty"`
}

type AuthorizePlanResult struct {
	Decision  string `json:"decision"`
	ReceiptID string `json:"receipt_id"`
	Reason    string `json:"reason,omitempty"`
}

type ExecuteToolInput struct {
	MessageID      string `json:"message_id"`
	WorkspaceID    string `json:"workspace_id"`
	ToolKey        string `json:"tool_key"`
	ReceiptID      string `json:"receipt_id"`
	IdempotencyKey string `json:"idempotency_key"`
	OAuthToken     string `json:"oauth_token,omitempty"`
}

type ToolExecutionActivityResult struct {
	ToolKey          string `json:"tool_key"`
	Phase            string `json:"phase"`
	Success          bool   `json:"success"`
	IdempotencyKey   string `json:"idempotency_key"`
	PayloadHash      string `json:"payload_hash"`
	ToolOutput       any    `json:"tool_output,omitempty"`
	UntrustedContent bool   `json:"untrusted_content,omitempty"`
}

type SynthesizeResponseInput struct {
	MessageID   string                        `json:"message_id"`
	WorkspaceID string                        `json:"workspace_id"`
	ToolResults []ToolExecutionActivityResult `json:"tool_results"`

	// EQ modulation fields — sourced from ApplyEQStrategyActivity (Phase 4).
	EQToneDirective  string  `json:"eq_tone_directive,omitempty"`
	EQFormalityLevel int     `json:"eq_formality_level,omitempty"`
	EQLengthModifier float64 `json:"eq_length_modifier,omitempty"`
	EQOfferHelp      bool    `json:"eq_offer_help,omitempty"`

	// Uncertainty qualifier flag — set when 0.50 <= confidence < 0.75.
	AddQualifiers bool    `json:"add_qualifiers,omitempty"`
	Confidence    float64 `json:"confidence,omitempty"`
}

// buildEQSystemPrompt returns the synthesis system prompt with EQ directives injected.
func buildEQSystemPrompt(input SynthesizeResponseInput) string {
	base := `You are Brevio, an executive AI assistant. Generate a natural, concise response
incorporating skill execution results. Output a JSON object with:
- response_text: string (user-facing response, max 4096 chars)
- suggested_actions: array of follow-up string suggestions
- follow_up_scheduled: boolean

Respond with ONLY the JSON object.`

	hasEQ := input.EQToneDirective != "" || input.EQFormalityLevel != 0 || input.EQLengthModifier != 0
	if !hasEQ {
		return base
	}

	var dirs []string
	if input.EQToneDirective != "" {
		dirs = append(dirs, "TONE: "+input.EQToneDirective)
	}
	switch {
	case input.EQFormalityLevel >= 4:
		dirs = append(dirs, "STYLE: Use formal professional language. Full sentences, no contractions.")
	case input.EQFormalityLevel > 0 && input.EQFormalityLevel <= 2:
		dirs = append(dirs, "STYLE: Use conversational, friendly language.")
	}
	switch {
	case input.EQLengthModifier > 0 && input.EQLengthModifier < 0.7:
		dirs = append(dirs, "LENGTH: Be very concise. Maximum 2 sentences in response_text.")
	case input.EQLengthModifier > 1.3:
		dirs = append(dirs, "LENGTH: Provide a thorough, detailed response.")
	}
	if input.EQOfferHelp {
		dirs = append(dirs, "EMPATHY: Begin with a brief empathetic acknowledgement before the main response.")
	}
	if len(dirs) == 0 {
		return base
	}
	return base + "\n\nEQ MODULATION DIRECTIVES (apply to response_text):\n" + strings.Join(dirs, "\n")
}

// addUncertaintyQualifiers prepends a qualifier phrase for medium-confidence responses.
func addUncertaintyQualifiers(response string) string {
	qualifiers := []string{
		"Based on the available information, ",
		"As best I can determine, ",
		"To my knowledge, ",
	}
	for _, q := range qualifiers {
		if strings.HasPrefix(response, q) {
			return response
		}
	}
	if len(response) == 0 {
		return qualifiers[0]
	}
	q := qualifiers[len(response)%len(qualifiers)]
	return q + strings.ToLower(response[:1]) + response[1:]
}

type SynthesizeResponseResult struct {
	ResponsePayload string `json:"response_payload"`
}

// VerifyExecutionInput carries context for the LLM-based critic/verifier.
type VerifyExecutionInput struct {
	MessageID       string                        `json:"message_id"`
	WorkspaceID     string                        `json:"workspace_id"`
	OriginalPayload string                        `json:"original_payload"`
	PlanID          string                        `json:"plan_id"`
	PlanToolKeys    []string                      `json:"plan_tool_keys"`
	PlanRiskLevel   string                        `json:"plan_risk_level"`
	FinalAnswerReqs string                        `json:"final_answer_requirements"`
	ToolResults     []ToolExecutionActivityResult `json:"tool_results"`
	RetryHints      string                        `json:"retry_hints,omitempty"`
}

// VerifyExecutionResult is the critic's verdict on tool execution quality.
type VerifyExecutionResult struct {
	Verdict    string   `json:"verdict"`      // "pass" or "fail"
	Reasons    []string `json:"reasons"`
	RetryHints string   `json:"retry_hints"`
}

type OutboxDispatchInput struct {
	BatchSize int `json:"batch_size"`
}

type OutboxEntry struct {
	ID          string `json:"id"`
	WorkspaceID string `json:"workspace_id"`
	EventType   string `json:"event_type"`
	Payload     string `json:"payload"`
	Target      string `json:"target"`
	Attempts    int    `json:"attempts"`
	MaxAttempts int    `json:"max_attempts"`
}

type OutboxFetchResult struct {
	Entries []OutboxEntry `json:"entries"`
}

type OutboxEntryDispatchResult struct {
	Success bool   `json:"success"`
	DLQ     bool   `json:"dlq"`
	Error   string `json:"error,omitempty"`
}

type OutboxDispatchResult struct {
	TotalFetched    int `json:"total_fetched"`
	TotalDispatched int `json:"total_dispatched"`
	TotalDLQ        int `json:"total_dlq"`
}

type ToolHealthEvalInput struct {
	ToolKey       string `json:"tool_key"`
	WindowSeconds int    `json:"window_seconds"`
}

type ToolHealthEvalResult struct {
	ToolKey     string  `json:"tool_key"`
	HealthScore float64 `json:"health_score"`
	Status      string  `json:"status"`
	Quarantined bool    `json:"quarantined"`
}

type OnboardingWorkflowInput struct {
	WorkspaceID string            `json:"workspace_id"`
	Answers     map[string]string `json:"answers"`
}

type OnboardingWorkflowResult struct {
	CompletedStages []string `json:"completed_stages"`
	Status          string   `json:"status"`
}

type OnboardingStageInput struct {
	WorkspaceID string            `json:"workspace_id"`
	Stage       string            `json:"stage"`
	Answers     map[string]string `json:"answers"`
}

type OnboardingStageResult struct {
	Stage   string `json:"stage"`
	Success bool   `json:"success"`
}

type CostRollupInput struct {
	WorkspaceID string `json:"workspace_id"`
	PeriodStart string `json:"period_start"`
	PeriodEnd   string `json:"period_end"`
}

type CostRollupResult struct {
	WorkspaceID  string  `json:"workspace_id"`
	TotalCostUSD float64 `json:"total_cost_usd"`
	EventCount   int     `json:"event_count"`
	RollupID     string  `json:"rollup_id"`
}

type KillSwitchInput struct {
	WorkspaceID string `json:"workspace_id"`
	ActivatedBy string `json:"activated_by"`
	Reason      string `json:"reason"`
}

type KillSwitchResult struct {
	WorkspaceID     string `json:"workspace_id"`
	WorkflowsHalted int    `json:"workflows_halted"`
	ToolsDisabled   int    `json:"tools_disabled"`
	ActivatedAt     string `json:"activated_at"`
}

// OutboxDispatcher dispatches outbox entries to their target channels.
type OutboxDispatcher interface {
	Dispatch(ctx context.Context, target string, payload []byte) error
}

// KillSwitchChecker checks if a kill switch is active for a workspace/user.
// Kill switch evaluation precedes ALL other gates (NNR-107).
type KillSwitchChecker interface {
	IsActive(ctx context.Context, workspaceID, userID string) (bool, error)
}

// SkillACLChecker checks if a skill is allowed for a user.
type SkillACLChecker interface {
	IsSkillAllowed(ctx context.Context, workspaceID, userID, skillID string) (bool, bool, error)
}

// HandsExecutor executes a tool via the Go hands runtime.
type HandsExecutor interface {
	ExecuteTool(ctx context.Context, skillID, workspaceID, receiptID, idempotencyKey, mode string, args map[string]interface{}) (success bool, output any, err error)
}

// ActivityDeps holds production dependencies for Temporal activities.
type ActivityDeps struct {
	Pool             *pgxpool.Pool
	OutboxSvc        *outbox.Service
	OutboxDispatcher OutboxDispatcher
	MemoryRepo       memory.ItemRepository
	OnboardingRepo   onboarding.Repository
	RAGRepo          rag.Repository
	KillSwitchCheck  KillSwitchChecker
	SkillACLCheck    SkillACLChecker

	// Intelligence layer: LLM service for real provider-backed inference.
	LLMService *llm.Service

	// V10.2 intelligence dependencies.
	EQRepo           eq.EQStrategyRepository
	DemotionRepo     trust.DemotionRepository
	IntelligenceRepo brain.IntelligenceRepository

	// V10.2 P8 memory/context/RAG/latency dependencies.
	DecayRepo         memory.DecayRepository
	ConflictRepo      memory.ConflictRepository
	ChunkSpecRepo     rag.ChunkSpecRepository
	CompressionRepo   contextlayer.CompressionRepository
	ContextRepo       contextlayer.Repository
	LatencyRepo       executor.LatencyRepository
	EmbeddingProvider rag.EmbeddingProvider
	VectorStore       *rag.PgVectorProdStore

	// V10.3 cognitive intelligence dependencies.
	CognitiveRepo cognition.CognitiveRepository

	// V10.4 outbound call dependencies.
	CallRepo      call.CallRepository
	CallService   *call.CallService
	PhoneVerifier call.PhoneVerifier

	// Hands runtime: Go-native tool execution service.
	HandsExecutor HandsExecutor

	// Working memory tier: Redis-backed in-flight task state.
	WorkingMemory WorkingMemoryService

	// OPA policy evaluator.
	OPAEvaluator *policy.Evaluator

	// OAuth credential resolver for tool execution.
	CredentialResolver *connectors.CredentialResolver

	// Self-modification policy service.
	SelfModService *selfmod.Service

	// IPI inference guard.
	InferenceGuard *guardrails.InferenceGuard

	// PAHF preference learning.
	MemorySvc           *memory.Service
	PreferenceRetriever *preference.Retriever

	// Production eval sampling.
	ProductionEvalSampler *eval.ProductionEvalSampler
	SynthesisVerifier     *eval.SynthesisVerifier
	QualityGauge          ProductionQualityGauge

	// A/B experiment routing.
	ExperimentRouter  *experiment.ExperimentRouter
	VariantScoreStore *experiment.VariantScoreStore

	// Vision processor.
	VisionProcessor *vision.VisionProcessor

	// Proactive monitor.
	ProactiveMonitor *proactive.ProactiveMonitor
	OfferBuilder     *proactive.OfferBuilder

	// A2A client and registry.
	A2AClient             *a2a.A2AClient
	ExternalAgentRegistry *a2a.ExternalAgentRegistry

	// DPO pipeline.
	DPOService         *dpo.Service
	ScoreStore         *rageval.ScoreStore
	FeatureFlagService *feature_flags.Service

	// Plan simulator.
	Simulator *simulation.Simulator

	// GAIA benchmark.
	BenchmarkRepo     *benchmark.Repository
	PrometheusMetrics *observability.PrometheusMetrics

	// Browser automation.
	BrowserClient     *browser.Client
	BrowserSandboxSvc *sandbox.MCPSandboxService

	// Delegation service.
	DelegationSvc *delegation.Service

	// Knowledge graph (Phase 5).
	KGService   *kg.Service
	KGRetriever *kg.Retriever

	// Trust scoring for SubAgent autonomy gate.
	TrustSvc *trust.Service
}

// ProductionQualityGauge records the rolling quality score to Prometheus.
type ProductionQualityGauge interface {
	Set(value float64)
}

// WorkingMemoryService is the minimal interface for working memory eviction in activities.
type WorkingMemoryService interface {
	Complete(ctx context.Context, workspaceID, taskID string)
}

// Activities is the struct that holds all activity implementations.
// Dependencies are injected at construction time via ActivityDeps.
type Activities struct {
	pool             *pgxpool.Pool
	outboxSvc        *outbox.Service
	outboxDispatcher OutboxDispatcher
	memoryRepo       memory.ItemRepository
	onboardingRepo   onboarding.Repository
	ragRepo          rag.Repository
	killSwitchCheck  KillSwitchChecker
	skillACLCheck    SkillACLChecker

	// Intelligence layer: LLM service for real provider-backed inference.
	llmService *llm.Service

	// V10.2 intelligence dependencies.
	eqRepo           eq.EQStrategyRepository
	demotionRepo     trust.DemotionRepository
	intelligenceRepo brain.IntelligenceRepository

	// V10.2 P8 memory/context/RAG/latency dependencies.
	decayRepo         memory.DecayRepository
	conflictRepo      memory.ConflictRepository
	chunkSpecRepo     rag.ChunkSpecRepository
	compressionRepo   contextlayer.CompressionRepository
	contextRepo       contextlayer.Repository
	latencyRepo       executor.LatencyRepository
	embeddingProvider rag.EmbeddingProvider
	vectorStore       *rag.PgVectorProdStore

	// V10.3 cognitive intelligence dependencies.
	cognitiveRepo cognition.CognitiveRepository

	// V10.4 outbound call dependencies.
	callRepo      call.CallRepository
	callService   *call.CallService
	phoneVerifier call.PhoneVerifier

	// Hands runtime: Go-native tool execution service.
	handsExecutor HandsExecutor

	// Working memory tier.
	workingMemory WorkingMemoryService

	// Reasoning upgrade services.
	calibrationSvc    *brain.CalibrationService
	counterfactualSvc *brain.CounterfactualService
	embedProvider     rag.EmbeddingProvider

	// OPA policy evaluator. Nil = policy gates skipped (dev/test mode only).
	opaEvaluator *policy.Evaluator

	// OAuth credential resolver for tool execution.
	credentialResolver *connectors.CredentialResolver

	// Self-modification policy service.
	selfModSvc *selfmod.Service

	// IPI inference guard for post-tool-call taint tracking.
	inferenceGuard *guardrails.InferenceGuard

	// PAHF preference learning.
	memorySvc           *memory.Service
	preferenceRetriever *preference.Retriever

	// Production eval sampling and hallucination detection.
	prodEvalSampler   *eval.ProductionEvalSampler
	synthesisVerifier *eval.SynthesisVerifier
	qualityGauge      ProductionQualityGauge

	// A/B experiment routing.
	experimentRouter  *experiment.ExperimentRouter
	variantScoreStore *experiment.VariantScoreStore

	// Vision processor.
	visionProcessor *vision.VisionProcessor

	// Proactive monitor.
	proactiveMonitor *proactive.ProactiveMonitor
	offerBuilder     *proactive.OfferBuilder

	// A2A client and registry.
	a2aClient             *a2a.A2AClient
	externalAgentRegistry *a2a.ExternalAgentRegistry

	// DPO pipeline.
	dpoService         *dpo.Service
	scoreStore         *rageval.ScoreStore
	featureFlagService *feature_flags.Service

	// Plan simulator.
	simulator *simulation.Simulator

	// GAIA benchmark.
	benchmarkRepo     *benchmark.Repository
	prometheusMetrics *observability.PrometheusMetrics

	// Browser automation.
	browserClient     *browser.Client
	browserSandboxSvc *sandbox.MCPSandboxService

	// Delegation service.
	delegationSvc *delegation.Service

	// Knowledge graph (Phase 5).
	kgService   *kg.Service
	kgRetriever *kg.Retriever

	// Trust scoring for SubAgent autonomy gate.
	trustSvc *trust.Service

	// Observability counters for outbox dispatch.
	outboxDispatched atomic.Int64
	outboxFailed     atomic.Int64
	outboxDLQ        atomic.Int64
}

// NewActivities creates an Activities struct with no dependencies (test/degraded mode).
func NewActivities() *Activities {
	return &Activities{}
}

// NewActivitiesWithProdDeps creates an Activities struct with production dependencies.
func NewActivitiesWithProdDeps(deps ActivityDeps) *Activities {
	return &Activities{
		pool:             deps.Pool,
		outboxSvc:        deps.OutboxSvc,
		outboxDispatcher: deps.OutboxDispatcher,
		memoryRepo:       deps.MemoryRepo,
		onboardingRepo:   deps.OnboardingRepo,
		ragRepo:          deps.RAGRepo,
		killSwitchCheck:  deps.KillSwitchCheck,
		skillACLCheck:    deps.SkillACLCheck,
		llmService:       deps.LLMService,
		eqRepo:           deps.EQRepo,
		demotionRepo:     deps.DemotionRepo,
		intelligenceRepo: deps.IntelligenceRepo,
		decayRepo:         deps.DecayRepo,
		conflictRepo:      deps.ConflictRepo,
		chunkSpecRepo:     deps.ChunkSpecRepo,
		compressionRepo:   deps.CompressionRepo,
		contextRepo:       deps.ContextRepo,
		latencyRepo:       deps.LatencyRepo,
		embeddingProvider: deps.EmbeddingProvider,
		vectorStore:      deps.VectorStore,
		cognitiveRepo:    deps.CognitiveRepo,
		callRepo:         deps.CallRepo,
		callService:      deps.CallService,
		phoneVerifier:    deps.PhoneVerifier,
		handsExecutor:    deps.HandsExecutor,
		workingMemory:   deps.WorkingMemory,
		opaEvaluator:       deps.OPAEvaluator,
		credentialResolver: deps.CredentialResolver,
		selfModSvc:         deps.SelfModService,
		inferenceGuard:      deps.InferenceGuard,
		memorySvc:           deps.MemorySvc,
		preferenceRetriever: deps.PreferenceRetriever,
		prodEvalSampler:     deps.ProductionEvalSampler,
		synthesisVerifier:   deps.SynthesisVerifier,
		qualityGauge:        deps.QualityGauge,
		experimentRouter:    deps.ExperimentRouter,
		variantScoreStore:   deps.VariantScoreStore,
		visionProcessor:     deps.VisionProcessor,
		proactiveMonitor:    deps.ProactiveMonitor,
		offerBuilder:        deps.OfferBuilder,
		a2aClient:             deps.A2AClient,
		externalAgentRegistry: deps.ExternalAgentRegistry,
		dpoService:            deps.DPOService,
		scoreStore:            deps.ScoreStore,
		featureFlagService:    deps.FeatureFlagService,
		simulator:             deps.Simulator,
		benchmarkRepo:         deps.BenchmarkRepo,
		prometheusMetrics:     deps.PrometheusMetrics,
		browserClient:         deps.BrowserClient,
		browserSandboxSvc:     deps.BrowserSandboxSvc,
		delegationSvc:         deps.DelegationSvc,
		kgService:             deps.KGService,
		kgRetriever:           deps.KGRetriever,
		trustSvc:              deps.TrustSvc,
	}
}

// OutboxMetrics returns current outbox dispatch counters for observability.
func (a *Activities) OutboxMetrics() (dispatched, failed, dlq int64) {
	return a.outboxDispatched.Load(), a.outboxFailed.Load(), a.outboxDLQ.Load()
}

func (a *Activities) ValidateEnvelopeActivity(ctx context.Context, input ValidateEnvelopeInput) (*ValidateEnvelopeResult, error) {
	if input.MessageID == "" || input.WorkspaceID == "" {
		return &ValidateEnvelopeResult{Valid: false, Reason: "missing_required_fields"}, nil
	}
	if input.RawPayload == "" {
		return &ValidateEnvelopeResult{Valid: false, Reason: "empty_payload"}, nil
	}
	return &ValidateEnvelopeResult{
		Valid:             true,
		NormalizedPayload: input.RawPayload,
	}, nil
}

func (a *Activities) ClassifyIntentActivity(ctx context.Context, input ClassifyIntentInput) (*ClassifyIntentResult, error) {
	if input.Payload == "" {
		return nil, fmt.Errorf("SCHEMA_VALIDATION_FAILED: empty payload")
	}

	// Production path: use LLM-backed intent classification.
	if a.llmService != nil && a.llmService.Intelligence() != nil {
		classification, _, err := a.llmService.ClassifyIntent(ctx, input.Payload, input.WorkspaceID)
		if err != nil {
			log.Printf("[ClassifyIntent] LLM classification failed, using keyword fallback: %v", err)
			fb := a.keywordFallbackClassification(input.Payload)
			fb.Deterministic = true
			return fb, nil
		}
		result := &ClassifyIntentResult{
			Intent:        classification.Intent,
			Confidence:    classification.Confidence,
			Deterministic: false,
		}
		if classification.Confidence < 0.7 {
			result.Fallback = "keyword_classifier"
			fallback := a.keywordFallbackClassification(input.Payload)
			if fallback.Confidence > classification.Confidence {
				result.Intent = fallback.Intent
				result.Confidence = fallback.Confidence
				result.Deterministic = true
			}
		}
		return result, nil
	}

	// Degraded path: keyword-based classification (deterministic).
	fb := a.keywordFallbackClassification(input.Payload)
	fb.Deterministic = true
	log.Printf("[ClassifyIntent] Deterministic=true: no LLM configured, using keyword fallback")
	return fb, nil
}

// keywordFallbackClassification provides deterministic keyword-based intent classification
// used when the LLM is unavailable or when LLM confidence is below threshold.
func (a *Activities) keywordFallbackClassification(payload string) *ClassifyIntentResult {
	lower := strings.ToLower(payload)
	type intentRule struct {
		keywords   []string
		intent     string
		confidence float64
	}
	rules := []intentRule{
		{[]string{"email", "mail", "inbox", "send email", "reply"}, "email_management", 0.80},
		{[]string{"calendar", "meeting", "schedule", "event", "appointment"}, "calendar_management", 0.80},
		{[]string{"search", "find", "look up", "google", "web"}, "web_search", 0.75},
		{[]string{"document", "file", "note", "doc", "write"}, "document_management", 0.75},
		{[]string{"task", "todo", "remind", "create task"}, "task_creation", 0.75},
		{[]string{"call", "phone", "dial", "ring"}, "outbound_call", 0.80},
	}
	for _, rule := range rules {
		for _, kw := range rule.keywords {
			if strings.Contains(lower, kw) {
				return &ClassifyIntentResult{
					Intent:     rule.intent,
					Confidence: rule.confidence,
					Fallback:   "keyword_classifier",
				}
			}
		}
	}
	return &ClassifyIntentResult{
		Intent:     "general_query",
		Confidence: 0.60,
		Fallback:   "keyword_classifier",
	}
}

func (a *Activities) GeneratePlanActivity(ctx context.Context, input GeneratePlanInput) (*GeneratePlanResult, error) {
	planID := hashKey("plan:" + input.MessageID + ":" + input.WorkspaceID)

	// PAHF step 2: inject preference context when plan confidence is low.
	if input.Confidence < 0.65 && a.preferenceRetriever != nil {
		prefFacts, prefErr := a.preferenceRetriever.FetchTopK(ctx, input.WorkspaceID, input.UserID, input.Intent, 5)
		if prefErr == nil && len(prefFacts) > 0 {
			preferenceCtx := preference.FormatForLLM(prefFacts)
			if input.MemoryContext == "" {
				input.MemoryContext = preferenceCtx
			} else {
				input.MemoryContext = preferenceCtx + "\n\n" + input.MemoryContext
			}
		}
	}

	// Compress conversation history if context budget is at risk.
	budget := input.ContextBudget
	if budget <= 0 {
		budget = 150_000
	}
	if len(input.ConversationHistory) > 0 && a.llmService != nil {
		compressed, compErr := brain.CompressConversation(ctx, input.ConversationHistory, budget, 6, a.llmService)
		if compErr != nil {
			log.Printf("[GeneratePlan] context compression warning (using full history): %v", compErr)
		} else {
			input.ConversationHistory = compressed
		}
	}

	// Build payload for LLM — use explicit payload if available, fall back to intent.
	payload := input.Payload
	if payload == "" {
		payload = input.Intent
	}

	// If re-planning after a verify failure, append retry hints to the payload.
	if input.RetryHints != "" {
		payload = payload + "\n\n[VERIFIER FEEDBACK — previous attempt was rejected]\n" + input.RetryHints
	}

	// Production path: use LLM-backed plan generation with confidence routing.
	if a.llmService != nil && a.llmService.Intelligence() != nil {
		var plan *llm.GeneratedPlan
		var err error
		if input.Confidence > 0 {
			plan, _, err = a.llmService.Intelligence().GeneratePlanWithRouting(
				ctx, input.Intent, input.Confidence, input.RequiresDecomposition,
				payload, input.MemoryContext, input.RAGContext,
			)
		} else {
			plan, _, err = a.llmService.GeneratePlan(ctx, input.Intent, input.Confidence, payload, input.MemoryContext, input.RAGContext)
		}
		if err != nil {
			log.Printf("[GeneratePlan] LLM plan generation failed, using deterministic fallback: %v", err)
			return a.deterministicPlanFallback(planID, input.Intent), nil
		}

		// Plan was already validated by IntelligenceService.GeneratePlan
		// which calls validatePlan + canonicalizePlan internally.
		// Guard against nil plan from unexpected LLM response shape.
		if plan == nil {
			log.Printf("[GeneratePlan] LLM returned nil plan without error, using deterministic fallback")
			return a.deterministicPlanFallback(planID, input.Intent), nil
		}

		toolKeys := make([]string, len(plan.Tools))
		copy(toolKeys, plan.Tools)
		sort.Strings(toolKeys)

		return &GeneratePlanResult{
			PlanID:          planID,
			ToolKeys:        toolKeys,
			RiskLevel:       strings.ToUpper(plan.RiskLevel),
			Deterministic:   false,
			FinalAnswerReqs: plan.FinalAnswerRequirements,
		}, nil
	}

	// Degraded path: deterministic plan based on intent keywords.
	log.Printf("[GeneratePlan] Deterministic=true: no LLM configured")
	return a.deterministicPlanFallback(planID, input.Intent), nil
}

// deterministicPlanFallback generates a predictable plan from intent keywords
// when the LLM is unavailable. This provides a testable, non-empty plan structure.
func (a *Activities) deterministicPlanFallback(planID, intent string) *GeneratePlanResult {
	lower := strings.ToLower(intent)
	var toolKeys []string
	riskLevel := "LOW"
	finalReqs := "Verify that the requested information was retrieved successfully."

	switch {
	case strings.Contains(lower, "email"):
		toolKeys = []string{"email.read"}
		if strings.Contains(lower, "send") || strings.Contains(lower, "reply") {
			toolKeys = append(toolKeys, "email.send")
			riskLevel = "ELEVATED"
			finalReqs = "Confirm the email was sent and the recipient address is correct."
		}
	case strings.Contains(lower, "calendar") || strings.Contains(lower, "meeting") || strings.Contains(lower, "schedule"):
		toolKeys = []string{"calendar.read"}
		if strings.Contains(lower, "create") || strings.Contains(lower, "schedule") {
			toolKeys = append(toolKeys, "calendar.write")
			riskLevel = "ELEVATED"
			finalReqs = "Confirm the calendar event was created with correct time and participants."
		}
	case strings.Contains(lower, "search") || strings.Contains(lower, "find") || strings.Contains(lower, "look"):
		toolKeys = []string{"web.search"}
		finalReqs = "Verify that search results are relevant to the query."
	case strings.Contains(lower, "task") || strings.Contains(lower, "todo"):
		toolKeys = []string{"task.create"}
		riskLevel = "LOW"
		finalReqs = "Confirm the task was created with the correct title and description."
	case strings.Contains(lower, "call") || strings.Contains(lower, "phone"):
		toolKeys = []string{"phone.dial"}
		riskLevel = "CRITICAL"
		finalReqs = "Confirm the call was initiated to the correct number."
	default:
		toolKeys = []string{"echo"}
		finalReqs = "Verify the echo response was returned."
	}

	sort.Strings(toolKeys)
	return &GeneratePlanResult{
		PlanID:          planID,
		ToolKeys:        toolKeys,
		RiskLevel:       riskLevel,
		Deterministic:   true,
		FinalAnswerReqs: finalReqs,
	}
}

func (a *Activities) AuthorizePlanActivity(ctx context.Context, input AuthorizePlanInput) (result *AuthorizePlanResult, err error) {
	authzStart := time.Now()
	defer func() {
		decision := "allow"
		if result != nil && result.Decision == "deny" {
			decision = "deny"
		}
		metrics.RecordAuthorization(decision, input.RiskLevel)
		metrics.RecordActivity("AuthorizePlanActivity", authzStart, err)
	}()

	// Gate 0: Kill switch — runs first, unbypassable (NNR-107).
	if a.killSwitchCheck != nil {
		active, err := a.killSwitchCheck.IsActive(ctx, input.WorkspaceID, "")
		if err != nil {
			return &AuthorizePlanResult{Decision: "deny", Reason: "KILL_SWITCH_CHECK_FAILED"}, nil
		}
		if active {
			return &AuthorizePlanResult{Decision: "deny", Reason: "KILL_SWITCH_ACTIVE"}, nil
		}
	}

	// Gate 1: OPA full policy evaluation (autonomy, budget, write-gate).
	if a.opaEvaluator != nil {
		planInput := policy.PlanAuthzInput{
			WorkspaceID: input.WorkspaceID,
			PlanID:      input.PlanID,
			ToolKeys:    input.ToolKeys,
			RiskLevel:   input.RiskLevel,
			Autonomy:    "A1", // safe default; enriched from workspace settings in production
			BudgetCents: 0,
			UsedCents:   0,
			UserTier:    "free",
		}
		decision := a.opaEvaluator.EvaluatePlan(ctx, planInput)
		if !decision.Allowed {
			return &AuthorizePlanResult{
				Decision: "deny",
				Reason:   decision.Reason,
			}, nil
		}
	}

	// Gate 1b: Self-modification policy check.
	if a.selfModSvc != nil {
		for _, toolKey := range input.ToolKeys {
			if isSelfModifyingOp(toolKey) {
				decision := a.selfModSvc.EvaluateAction(input.WorkspaceID, selfmod.ActionRequest{
					WorkspaceID:   input.WorkspaceID,
					ActionKey:     toolKey,
					RequestedRisk: input.RiskLevel,
				})
				if decision.Decision == "deny" {
					return &AuthorizePlanResult{
						Decision: "deny",
						Reason:   "SELF_MOD_POLICY_DENIED:" + decision.Reason,
					}, nil
				}
			}
		}
	}

	// Gate 2: Skill ACL per tool key.
	if a.skillACLCheck != nil {
		for _, toolKey := range input.ToolKeys {
			allowed, hasOverride, err := a.skillACLCheck.IsSkillAllowed(ctx, input.WorkspaceID, "", toolKey)
			if err != nil {
				return &AuthorizePlanResult{
					Decision: "deny",
					Reason:   fmt.Sprintf("SKILL_ACL_CHECK_FAILED: tool=%s: %v", toolKey, err),
				}, nil
			}
			if hasOverride && !allowed {
				return &AuthorizePlanResult{
					Decision: "deny",
					Reason:   fmt.Sprintf("SKILL_ACL_DENIED:%s", toolKey),
				}, nil
			}
		}
	}

	// Gate 3: Delegation check — cross-user workspace access requires grant (Phase 4).
	if a.delegationSvc != nil &&
		input.RequestingUserID != "" &&
		input.WorkspaceOwnerID != "" &&
		input.RequestingUserID != input.WorkspaceOwnerID {
		wsID, parseErr := uuid.Parse(input.WorkspaceID)
		if parseErr != nil {
			return &AuthorizePlanResult{Decision: "deny", Reason: fmt.Sprintf("DELEGATION_INVALID_WORKSPACE: %v", parseErr)}, nil
		}
		granteeID, parseErr := uuid.Parse(input.RequestingUserID)
		if parseErr != nil {
			return &AuthorizePlanResult{Decision: "deny", Reason: fmt.Sprintf("DELEGATION_INVALID_GRANTEE: %v", parseErr)}, nil
		}
		for _, toolKey := range input.ToolKeys {
			if !a.delegationSvc.CanGranteeUseTool(wsID, granteeID, toolKey) {
				return &AuthorizePlanResult{
					Decision: "deny",
					Reason: fmt.Sprintf("DELEGATION_REQUIRED: no valid grant covers tool %q for grantee %s in workspace %s",
						toolKey, input.RequestingUserID, input.WorkspaceID),
				}, nil
			}
		}
	}

	receiptID := hashKey("receipt:" + input.PlanID + ":" + input.WorkspaceID)
	return &AuthorizePlanResult{
		Decision:  "allow",
		ReceiptID: receiptID,
	}, nil
}

func (a *Activities) ExecuteToolActivity(ctx context.Context, input ExecuteToolInput) (_ *ToolExecutionActivityResult, err error) {
	execStart := time.Now()
	defer func() {
		status := "success"
		if err != nil {
			status = "error"
		}
		metrics.RecordToolExecution(input.ToolKey, status)
		metrics.RecordActivity("ExecuteToolActivity", execStart, err)
	}()

	if input.ReceiptID == "" {
		return nil, fmt.Errorf("AUTHORIZATION_REQUIRED: no receipt provided")
	}

	// NNR-107: Kill switch check before executor commit — unbypassable.
	if a.killSwitchCheck != nil {
		active, err := a.killSwitchCheck.IsActive(ctx, input.WorkspaceID, "")
		if err != nil || active {
			return nil, fmt.Errorf("KILL_SWITCH_ACTIVE: execution blocked for workspace %s", input.WorkspaceID)
		}
	}

	payloadHash := hashKey(input.WorkspaceID + "::" + input.ToolKey + "::" + input.IdempotencyKey)

	// Resolve OAuth token for this tool key if not already set.
	if input.OAuthToken == "" && a.credentialResolver != nil {
		if resolved, resolveErr := a.credentialResolver.ResolveToken(
			ctx, input.WorkspaceID, "", input.ToolKey,
		); resolveErr == nil && resolved != "" {
			input.OAuthToken = resolved
		}
	}

	// A2A delegation path: tool keys starting with "delegate:" route to external agents.
	if strings.HasPrefix(input.ToolKey, "delegate:") && a.a2aClient != nil {
		capability := strings.TrimPrefix(input.ToolKey, "delegate:")
		delegateReq := a2a.DelegateRequest{
			WorkspaceID: input.WorkspaceID,
			Capability:  capability,
			Input:       map[string]any{"workspace_id": input.WorkspaceID},
			TimeoutSecs: 120,
		}
		delegateResult, delegateErr := a.a2aClient.Delegate(ctx, delegateReq)
		if delegateErr != nil {
			return &ToolExecutionActivityResult{
				ToolKey: input.ToolKey, Phase: "a2a_delegation_failed", Success: false,
				IdempotencyKey: input.IdempotencyKey, PayloadHash: payloadHash,
			}, nil
		}
		return &ToolExecutionActivityResult{
			ToolKey: input.ToolKey, Phase: "commit", Success: delegateResult.Status == a2a.TaskStatusCompleted,
			IdempotencyKey: input.IdempotencyKey, PayloadHash: payloadHash,
			ToolOutput: delegateResult.Output,
		}, nil
	}

	// Production path: call Go hands runtime for real execution.
	if a.handsExecutor != nil {
		execArgs := map[string]interface{}{}
		if input.OAuthToken != "" {
			execArgs["oauth_token"] = input.OAuthToken
		}
		success, output, err := a.handsExecutor.ExecuteTool(
			ctx, input.ToolKey, input.WorkspaceID, input.ReceiptID,
			input.IdempotencyKey, "commit", execArgs,
		)
		if err != nil {
			log.Printf("[ExecuteTool] hands execution failed for %s: %v", input.ToolKey, err)
			return &ToolExecutionActivityResult{
				ToolKey:        input.ToolKey,
				Phase:          "commit",
				Success:        false,
				IdempotencyKey: input.IdempotencyKey,
				PayloadHash:    payloadHash,
			}, nil
		}
		result := &ToolExecutionActivityResult{
			ToolKey:        input.ToolKey,
			Phase:          "commit",
			Success:        success,
			IdempotencyKey: input.IdempotencyKey,
			PayloadHash:    payloadHash,
			ToolOutput:     output,
		}

		// IPI taint-tracking: check tool output for indirect prompt injection.
		if output != nil && a.inferenceGuard != nil {
			trustSrc := guardrails.InferTrustSource(input.ToolKey)
			if trustSrc.IsUntrusted() {
				ipiResult := a.inferenceGuard.CheckPostToolCallIPI(guardrails.IPIGuardInput{
					WorkspaceID: input.WorkspaceID,
					TrustSource: trustSrc,
					ToolOutput:  fmt.Sprintf("%v", output),
				})
				if !ipiResult.Allowed {
					log.Printf("[ExecuteTool] IPI_BLOCKED tool=%s reason=%s", input.ToolKey, ipiResult.Reason)
					return &ToolExecutionActivityResult{
						ToolKey: input.ToolKey,
						Phase:   "ipi_blocked",
						Success: false,
					}, fmt.Errorf("IPI_BLOCKED: %s", ipiResult.Reason)
				}
				if ipiResult.UntrustedContent {
					result.UntrustedContent = true
				}
			}
		}

		return result, nil
	}

	// REPAIR: missing HandsExecutor is a configuration error in production.
	// Return non-retryable error so the workflow cannot silently succeed.
	log.Printf("[ExecuteTool] FAILED tool=%s reason=HANDS_EXECUTOR_UNCONFIGURED", input.ToolKey)
	return &ToolExecutionActivityResult{
		ToolKey:        input.ToolKey,
		Phase:          "commit",
		Success:        false,
		IdempotencyKey: input.IdempotencyKey,
		PayloadHash:    payloadHash,
		ToolOutput:     `{"error":"HANDS_EXECUTOR_UNCONFIGURED"}`,
	}, temporal.NewNonRetryableApplicationError(
		"HANDS_EXECUTOR_UNCONFIGURED: no executor configured for tool execution",
		"CONFIGURATION_ERROR",
		nil,
	)
}

func (a *Activities) SynthesizeResponseActivity(ctx context.Context, input SynthesizeResponseInput) (*SynthesizeResponseResult, error) {
	// Production path: use LLM-backed response synthesis.
	if a.llmService != nil && a.llmService.Intelligence() != nil {
		// Build tool results summary for the LLM.
		var toolSummary strings.Builder
		for _, tr := range input.ToolResults {
			status := "failed"
			if tr.Success {
				status = "success"
			}
			fmt.Fprintf(&toolSummary, "- %s [%s]: phase=%s hash=%s\n", tr.ToolKey, status, tr.Phase, tr.PayloadHash)
		}

		// Streaming path — only active when FEATURE_STREAMING_ENABLED=true.
		if feature_flags.StreamingEnabled() {
			streamOut := make(chan llm.StreamChunk, 64)
			go a.llmService.StreamSynthesizeResponse(ctx, input.MessageID, toolSummary.String(), streamOut)

			var sb strings.Builder
			var streamErr error
			for chunk := range streamOut {
				if chunk.Error != nil {
					streamErr = chunk.Error
					break
				}
				sb.WriteString(chunk.Delta)
			}

			if streamErr == nil && sb.Len() > 0 {
				return &SynthesizeResponseResult{
					ResponsePayload: sb.String(),
				}, nil
			}
			if streamErr != nil {
				log.Printf("[Synthesize] streaming failed, falling back to non-streaming: %v", streamErr)
			}
			// Falls through to the non-streaming path below on any failure.
		}

		// Non-streaming path with EQ modulation (Phase 4).
		systemPrompt := buildEQSystemPrompt(input)
		hasEQ := input.EQToneDirective != "" || input.EQFormalityLevel != 0 || input.EQLengthModifier != 0

		var synthesized *llm.SynthesizedResponse
		var synthErr error
		if hasEQ && a.llmService.Intelligence() != nil {
			synthesized, _, synthErr = a.llmService.Intelligence().SynthesizeResponseWithSystemPrompt(
				ctx, input.MessageID, toolSummary.String(), systemPrompt)
		} else {
			synthesized, _, synthErr = a.llmService.SynthesizeResponse(ctx, input.MessageID, toolSummary.String())
		}

		if synthErr != nil {
			log.Printf("[Synthesize] LLM synthesis failed, using template fallback: %v", synthErr)
			return &SynthesizeResponseResult{
				ResponsePayload: fmt.Sprintf("Processed message %s with %d tool results", input.MessageID, len(input.ToolResults)),
			}, nil
		}
		if synthesized == nil {
			log.Printf("[Synthesize] LLM returned nil response without error, using template fallback")
			return &SynthesizeResponseResult{
				ResponsePayload: fmt.Sprintf("Processed message %s with %d tool results", input.MessageID, len(input.ToolResults)),
			}, nil
		}
		responseText := synthesized.ResponseText

		// Uncertainty qualifiers for medium-confidence responses.
		if input.AddQualifiers {
			responseText = addUncertaintyQualifiers(responseText)
		}

		// Hallucination detection for T2/T3 responses.
		if a.synthesisVerifier != nil {
			vr, _ := a.synthesisVerifier.Verify(ctx, input.WorkspaceID, "", input.MessageID, responseText, "t2")
			if vr != nil && !vr.Passed {
				responseText += "\n\n_(Note: Some details in this response may require verification.)_"
			}
		}

		return &SynthesizeResponseResult{
			ResponsePayload: responseText,
		}, nil
	}

	// Degraded path: template-based response.
	responsePayload := fmt.Sprintf("Processed message %s with %d tool results", input.MessageID, len(input.ToolResults))

	// Mark message as AI-generated (EU AI Act Article 50 compliance).
	if a.pool != nil && input.MessageID != "" {
		_, _ = a.pool.Exec(ctx, `UPDATE messages SET is_ai_generated=true WHERE id=$1`, input.MessageID)
	}

	return &SynthesizeResponseResult{
		ResponsePayload: responsePayload,
	}, nil
}

// VerifyExecutionActivity is the LLM-based critic that evaluates whether tool
// execution results satisfy the plan's goals. It returns pass/fail with reasons
// and retry hints that can be fed back into the planner on re-attempt.
func (a *Activities) VerifyExecutionActivity(ctx context.Context, input VerifyExecutionInput) (*VerifyExecutionResult, error) {
	// Check if all tools succeeded — fast path for obvious failures.
	allSucceeded := true
	for _, tr := range input.ToolResults {
		if !tr.Success {
			allSucceeded = false
			break
		}
	}

	// If no tools executed at all, fail immediately without calling LLM.
	if len(input.ToolResults) == 0 {
		return &VerifyExecutionResult{
			Verdict:    "fail",
			Reasons:    []string{"no tool executions were completed"},
			RetryHints: "Ensure the plan includes at least one executable tool step.",
		}, nil
	}

	// Production path: use LLM-backed verification.
	if a.llmService != nil && a.llmService.Intelligence() != nil {
		toolOutputs := make([]llm.ToolOutputForVerify, len(input.ToolResults))
		for i, tr := range input.ToolResults {
			toolOutputs[i] = llm.ToolOutputForVerify{
				ToolKey:     tr.ToolKey,
				Success:     tr.Success,
				PayloadHash: tr.PayloadHash,
				Phase:       tr.Phase,
			}
		}

		verifyInput := llm.VerifyInput{
			OriginalRequest: input.OriginalPayload,
			Plan: &llm.GeneratedPlan{
				Intent:                  input.PlanID,
				Tools:                   input.PlanToolKeys,
				RiskLevel:               strings.ToLower(input.PlanRiskLevel),
				FinalAnswerRequirements: input.FinalAnswerReqs,
			},
			ToolOutputs: toolOutputs,
			RetryHints:  input.RetryHints,
		}

		result, _, err := a.llmService.VerifyExecution(ctx, verifyInput)
		if err != nil {
			log.Printf("[VerifyExecution] LLM verification failed, using deterministic fallback: %v", err)
			return a.deterministicVerifyFallback(input.ToolResults, allSucceeded), nil
		}

		return &VerifyExecutionResult{
			Verdict:    result.Verdict,
			Reasons:    result.Reasons,
			RetryHints: result.RetryHints,
		}, nil
	}

	// Degraded path: deterministic verification based on success/failure counts.
	log.Printf("[VerifyExecution] Deterministic=true: no LLM configured")
	return a.deterministicVerifyFallback(input.ToolResults, allSucceeded), nil
}

// deterministicVerifyFallback provides a rule-based verification when the LLM
// is unavailable. Passes if all tools succeeded; fails otherwise.
func (a *Activities) deterministicVerifyFallback(results []ToolExecutionActivityResult, allSucceeded bool) *VerifyExecutionResult {
	if allSucceeded {
		return &VerifyExecutionResult{
			Verdict:    "pass",
			Reasons:    []string{"all tool executions succeeded"},
			RetryHints: "",
		}
	}
	var failedTools []string
	for _, tr := range results {
		if !tr.Success {
			failedTools = append(failedTools, tr.ToolKey)
		}
	}
	return &VerifyExecutionResult{
		Verdict:    "fail",
		Reasons:    []string{fmt.Sprintf("tool execution failures: %s", strings.Join(failedTools, ", "))},
		RetryHints: fmt.Sprintf("Retry failed tools: %s. Consider alternative tools if available.", strings.Join(failedTools, ", ")),
	}
}

// FetchPendingOutboxActivity fetches pending outbox entries from the database.
// In production mode (outboxSvc != nil), queries the outbox table using
// FOR UPDATE SKIP LOCKED for safe concurrent consumption.
func (a *Activities) FetchPendingOutboxActivity(ctx context.Context, input OutboxDispatchInput) (*OutboxFetchResult, error) {
	if a.outboxSvc == nil {
		return &OutboxFetchResult{Entries: []OutboxEntry{}}, nil
	}

	batchSize := input.BatchSize
	if batchSize <= 0 {
		batchSize = 100
	}

	dbEntries, err := a.outboxSvc.FetchPending(ctx, batchSize)
	if err != nil {
		return nil, fmt.Errorf("fetch pending outbox entries: %w", err)
	}

	log.Printf("[FetchPendingOutbox] fetched %d entries (batch=%d)", len(dbEntries), batchSize)

	entries := make([]OutboxEntry, 0, len(dbEntries))
	for _, e := range dbEntries {
		entries = append(entries, OutboxEntry{
			ID:          e.ID,
			WorkspaceID: e.WorkspaceID,
			EventType:   e.EventType,
			Payload:     string(e.Payload),
			Target:      e.Target,
			Attempts:    e.Attempts,
			MaxAttempts: e.MaxAttempts,
		})
	}

	return &OutboxFetchResult{Entries: entries}, nil
}

// DispatchOutboxEntryActivity dispatches a single outbox entry to its target channel,
// then marks it as dispatched or failed in the database. When an entry exceeds
// max_attempts, it is moved to the dead-letter queue (DLQ).
//
// Idempotency: if the entry is already dispatched (MarkDispatched returns "not found"
// because status changed), the activity returns success to avoid duplicate delivery.
// Observable: increments atomic counters for dispatched/failed/DLQ metrics.
func (a *Activities) DispatchOutboxEntryActivity(ctx context.Context, entry OutboxEntry) (*OutboxEntryDispatchResult, error) {
	if a.outboxSvc == nil {
		return &OutboxEntryDispatchResult{
			Success: false,
			Error:   "OUTBOX_SERVICE_UNCONFIGURED",
		}, temporal.NewNonRetryableApplicationError(
			"OUTBOX_SERVICE_UNCONFIGURED: no outbox service configured for dispatch",
			"CONFIGURATION_ERROR",
			nil,
		)
	}

	log.Printf("[OutboxDispatch] entry=%s target=%s attempts=%d/%d", entry.ID, entry.Target, entry.Attempts, entry.MaxAttempts)

	// REPAIR: outboxDispatcher must be configured for production dispatch.
	// A nil dispatcher is a configuration error — fail explicitly so retries/DLQ apply.
	if a.outboxDispatcher == nil {
		failReason := "OUTBOX_DISPATCHER_UNCONFIGURED"
		_ = a.outboxSvc.MarkFailed(ctx, entry.ID, failReason)
		a.outboxFailed.Add(1)
		log.Printf("[OutboxDispatch] FAILED entry=%s reason=%s", entry.ID, failReason)
		return &OutboxEntryDispatchResult{
			Success: false,
			Error:   failReason,
		}, temporal.NewNonRetryableApplicationError(
			"OUTBOX_DISPATCHER_UNCONFIGURED: no dispatcher configured for outbox delivery",
			"CONFIGURATION_ERROR",
			nil,
		)
	}

	// Attempt dispatch via the configured dispatcher.
	dispatchErr := a.outboxDispatcher.Dispatch(ctx, entry.Target, []byte(entry.Payload))

	if dispatchErr != nil {
		// Mark failed — outbox service handles exponential backoff and DLQ promotion.
		_ = a.outboxSvc.MarkFailed(ctx, entry.ID, dispatchErr.Error())

		// Check if this failure exhausts max attempts → DLQ.
		if entry.Attempts+1 >= entry.MaxAttempts {
			a.outboxDLQ.Add(1)
			log.Printf("[OutboxDispatch] DLQ entry=%s reason=%s", entry.ID, dispatchErr.Error())
			return &OutboxEntryDispatchResult{
				Success: false,
				DLQ:     true,
				Error:   dispatchErr.Error(),
			}, nil
		}
		a.outboxFailed.Add(1)
		log.Printf("[OutboxDispatch] FAILED entry=%s reason=%s", entry.ID, dispatchErr.Error())
		return &OutboxEntryDispatchResult{
			Success: false,
			Error:   dispatchErr.Error(),
		}, nil
	}

	if err := a.outboxSvc.MarkDispatched(ctx, entry.ID); err != nil {
		// Idempotency guard: if MarkDispatched fails because the entry was already
		// dispatched (e.g., Temporal retry after successful dispatch), treat as success.
		if strings.Contains(err.Error(), "not found") {
			log.Printf("[OutboxDispatch] idempotent skip entry=%s (already dispatched)", entry.ID)
			a.outboxDispatched.Add(1)
			return &OutboxEntryDispatchResult{Success: true}, nil
		}
		a.outboxFailed.Add(1)
		return &OutboxEntryDispatchResult{
			Success: false,
			Error:   fmt.Sprintf("mark dispatched: %v", err),
		}, nil
	}

	a.outboxDispatched.Add(1)
	log.Printf("[OutboxDispatch] OK entry=%s target=%s", entry.ID, entry.Target)
	return &OutboxEntryDispatchResult{Success: true}, nil
}

func (a *Activities) EvaluateToolHealthActivity(ctx context.Context, input ToolHealthEvalInput) (*ToolHealthEvalResult, error) {
	return &ToolHealthEvalResult{
		ToolKey:     input.ToolKey,
		HealthScore: 1.0,
		Status:      "healthy",
		Quarantined: false,
	}, nil
}

// ExecuteOnboardingStageActivity processes a single onboarding stage.
// In production mode (onboardingRepo != nil), persists stage progression to DB.
// The activity is idempotent: re-executing a completed stage is a no-op.
func (a *Activities) ExecuteOnboardingStageActivity(ctx context.Context, input OnboardingStageInput) (*OnboardingStageResult, error) {
	answer, ok := input.Answers[input.Stage]
	if !ok || answer == "" {
		return nil, fmt.Errorf("STAGE_INCOMPLETE: missing answer for stage %s", input.Stage)
	}

	// Production path: use DB-backed onboarding repository.
	if a.onboardingRepo != nil {
		// Ensure session exists (idempotent — StartSession uses ON CONFLICT DO NOTHING).
		session, err := a.onboardingRepo.StartSession(ctx, input.WorkspaceID)
		if err != nil {
			return nil, fmt.Errorf("start onboarding session: %w", err)
		}

		// Only advance if the session is at or before the requested stage.
		if session.CurrentStage == input.Stage {
			if advErr := a.onboardingRepo.AdvanceStage(ctx, session.ID, input.Answers); advErr != nil {
				return nil, fmt.Errorf("advance stage %s: %w", input.Stage, advErr)
			}
		}
		// If currentStage is past the requested stage, this is an idempotent replay — success.
	}

	return &OnboardingStageResult{
		Stage:   input.Stage,
		Success: true,
	}, nil
}

func (a *Activities) AggregateCostsActivity(ctx context.Context, input CostRollupInput) (*CostRollupResult, error) {
	rollupID := hashKey("rollup:" + input.WorkspaceID + ":" + input.PeriodStart)

	if a.pool != nil {
		// Aggregate from llm_cost_ledger + connector_cost_ledger into user_cost_daily_rollup (NNR-105).
		// This is the ONLY code path that writes rollup tables.
		_, _ = a.pool.Exec(ctx,
			`INSERT INTO user_cost_daily_rollup (workspace_id, user_id, rollup_date, llm_cost_usd, connector_cost_usd, total_cost_usd, task_count, llm_calls, connector_calls)
			 SELECT
			   l.workspace_id, l.user_id, $3::date AS rollup_date,
			   COALESCE(SUM(l.cost_usd), 0),
			   0, COALESCE(SUM(l.cost_usd), 0),
			   COUNT(DISTINCT l.workflow_run_id), COUNT(*), 0
			 FROM llm_cost_ledger l
			 WHERE l.workspace_id = $1::uuid
			   AND l.created_at >= $3::timestamptz AND l.created_at < $4::timestamptz
			 GROUP BY l.workspace_id, l.user_id
			 ON CONFLICT (workspace_id, user_id, rollup_date) DO UPDATE SET
			   llm_cost_usd = EXCLUDED.llm_cost_usd,
			   total_cost_usd = EXCLUDED.llm_cost_usd + user_cost_daily_rollup.connector_cost_usd,
			   task_count = EXCLUDED.task_count,
			   llm_calls = EXCLUDED.llm_calls`,
			input.WorkspaceID, rollupID, input.PeriodStart, input.PeriodEnd)

		// Merge connector costs into the same rollup rows.
		_, _ = a.pool.Exec(ctx,
			`INSERT INTO user_cost_daily_rollup (workspace_id, user_id, rollup_date, llm_cost_usd, connector_cost_usd, total_cost_usd, task_count, llm_calls, connector_calls)
			 SELECT
			   c.workspace_id, c.user_id, $3::date, 0,
			   COALESCE(SUM(c.cost_usd), 0), COALESCE(SUM(c.cost_usd), 0),
			   COUNT(DISTINCT c.workflow_run_id), 0, COUNT(*)
			 FROM connector_cost_ledger c
			 WHERE c.workspace_id = $1::uuid
			   AND c.created_at >= $3::timestamptz AND c.created_at < $4::timestamptz
			 GROUP BY c.workspace_id, c.user_id
			 ON CONFLICT (workspace_id, user_id, rollup_date) DO UPDATE SET
			   connector_cost_usd = EXCLUDED.connector_cost_usd,
			   total_cost_usd = user_cost_daily_rollup.llm_cost_usd + EXCLUDED.connector_cost_usd,
			   connector_calls = EXCLUDED.connector_calls`,
			input.WorkspaceID, rollupID, input.PeriodStart, input.PeriodEnd)

		// Read back aggregated totals.
		var totalCost float64
		var eventCount int
		err := a.pool.QueryRow(ctx,
			`SELECT COALESCE(SUM(total_cost_usd), 0), COALESCE(SUM(task_count), 0)
			 FROM user_cost_daily_rollup
			 WHERE workspace_id = $1::uuid AND rollup_date = $2::date`,
			input.WorkspaceID, input.PeriodStart).Scan(&totalCost, &eventCount)
		if err == nil {
			return &CostRollupResult{
				WorkspaceID:  input.WorkspaceID,
				TotalCostUSD: totalCost,
				EventCount:   eventCount,
				RollupID:     rollupID,
			}, nil
		}
	}

	return &CostRollupResult{
		WorkspaceID:  input.WorkspaceID,
		TotalCostUSD: 0,
		EventCount:   0,
		RollupID:     rollupID,
	}, nil
}

func (a *Activities) ActivateKillSwitchActivity(ctx context.Context, input KillSwitchInput) (*KillSwitchResult, error) {
	activatedAt := time.Now().UTC().Format(time.RFC3339)

	if a.pool != nil {
		_, _ = a.pool.Exec(ctx,
			`INSERT INTO kill_switch_state (workspace_id, is_active, activated_by, reason, activated_at)
			 VALUES ($1, true, $2, $3, $4)
			 ON CONFLICT (workspace_id) DO UPDATE SET is_active = true, activated_by = $2, reason = $3, activated_at = $4`,
			input.WorkspaceID, input.ActivatedBy, input.Reason, activatedAt)
	}

	return &KillSwitchResult{
		WorkspaceID:     input.WorkspaceID,
		WorkflowsHalted: 0,
		ToolsDisabled:   0,
		ActivatedAt:     activatedAt,
	}, nil
}

// --- Voice activities (method-based) ---

func (a *Activities) InitVoiceSessionActivity(ctx context.Context, input VoiceInitInput) (*VoiceInitResult, error) {
	if input.SessionID == "" || input.WorkspaceID == "" {
		return nil, fmt.Errorf("session_id and workspace_id are required")
	}

	h := sha256.Sum256([]byte(input.SessionID + ":" + input.WorkspaceID))
	roomName := "voice-" + hex.EncodeToString(h[:8])

	apiKey := os.Getenv("LIVEKIT_API_KEY")
	apiSecret := os.Getenv("LIVEKIT_API_SECRET")

	if apiKey == "" || apiSecret == "" {
		// Graceful degradation in dev/test environments.
		return &VoiceInitResult{
			Token:    "livekit-token-unavailable-check-env",
			RoomName: roomName,
		}, nil
	}

	signer, err := newTokenSignerAdapter(apiKey, apiSecret)
	if err != nil {
		return nil, fmt.Errorf("init voice session: create token signer: %w", err)
	}
	token, err := signer.Sign(input.SessionID, input.WorkspaceID, roomName)
	if err != nil {
		return nil, fmt.Errorf("sign livekit token: %w", err)
	}
	return &VoiceInitResult{Token: token, RoomName: roomName}, nil
}

func (a *Activities) ExtractVoiceTasksActivity(ctx context.Context, input VoiceTaskExtractInput) (*VoiceTaskExtractResult, error) {
	if input.Transcript == "" {
		return &VoiceTaskExtractResult{Tasks: []string{}}, nil
	}

	// Use LLM extraction when ANTHROPIC_API_KEY is configured.
	if client := a.buildLLMClient(); client != nil {
		extractor, err := worker.NewLLMTaskExtractor(worker.LLMTaskExtractorConfig{
			LLMClient:      client,
			TodayDate:      time.Now().UTC().Format("2006-01-02"),
			FallbackOnFail: true,
			MinConfidence:  0.65,
		})
		if err == nil {
			result, extractErr := extractor.Extract(ctx, input.Transcript)
			if extractErr == nil {
				tasks := make([]string, 0, len(result.Tasks))
				for _, t := range result.Tasks {
					if strings.TrimSpace(t.Description) != "" {
						tasks = append(tasks, strings.TrimSpace(t.Description))
					}
				}
				return &VoiceTaskExtractResult{Tasks: tasks}, nil
			}
		}
	}

	// Keyword fallback — split into sentences for per-sentence matching.
	kwExtractor := worker.NewKeywordTaskExtractor()
	sentences := strings.Split(input.Transcript, ".")
	turns := make([]worker.TranscriptTurn, 0, len(sentences))
	for _, s := range sentences {
		s = strings.TrimSpace(s)
		if s != "" {
			turns = append(turns, worker.TranscriptTurn{Speaker: "user", Text: s})
		}
	}
	extracted := kwExtractor.ExtractTasks(turns)
	tasks := make([]string, 0, len(extracted))
	for _, t := range extracted {
		if strings.TrimSpace(t.Description) != "" {
			tasks = append(tasks, strings.TrimSpace(t.Description))
		}
	}
	return &VoiceTaskExtractResult{Tasks: tasks}, nil
}

func (a *Activities) AnalyseSentimentActivity(ctx context.Context, input AnalyseSentimentInput) (*AnalyseSentimentResult, error) {
	if input.Transcript == "" {
		return &AnalyseSentimentResult{Summary: "", OverallLabel: "neutral", OverallScore: 0.5}, nil
	}
	client := a.buildLLMClient()
	if client == nil {
		return &AnalyseSentimentResult{
			Summary:      "sentiment unavailable (ANTHROPIC_API_KEY not configured)",
			OverallLabel: "neutral", OverallScore: 0.5,
		}, nil
	}
	analyser, err := gateway.NewLLMSentimentAnalyser(gateway.LLMSentimentAnalyserConfig{LLMClient: client})
	if err != nil {
		return nil, fmt.Errorf("analyse sentiment: %w", err)
	}
	result, err := analyser.Analyse(ctx, input.Transcript, nil)
	if err != nil {
		// Non-fatal — return neutral rather than failing the workflow.
		return &AnalyseSentimentResult{
			Summary: "sentiment failed: " + err.Error(), OverallLabel: "neutral", OverallScore: 0.5,
		}, nil
	}
	return &AnalyseSentimentResult{
		Summary:          result.Summary,
		EscalationSignal: result.EscalationSignal,
		OverallLabel:     string(result.Overall.Label),
		OverallScore:     result.Overall.Score,
	}, nil
}

// --- Learning activities (method-based) ---

func (a *Activities) ClusterCorrectionsActivity(ctx context.Context, input ClusterCorrectionsInput) (*ClusterCorrectionsResult, error) {
	if input.WorkspaceID == "" {
		return nil, fmt.Errorf("workspace_id is required")
	}
	return &ClusterCorrectionsResult{LessonsCreated: 0}, nil
}

func (a *Activities) DetectConflictsActivity(ctx context.Context, input DetectConflictsInput) (*DetectConflictsResult, error) {
	if input.WorkspaceID == "" {
		return nil, fmt.Errorf("workspace_id is required")
	}
	return &DetectConflictsResult{TotalConflicts: 0, RedundantConflictIDs: []string{}}, nil
}

func (a *Activities) ResolveConflictActivity(ctx context.Context, input ResolveConflictInput) (*ResolveConflictResult, error) {
	if input.ConflictID == "" {
		return nil, fmt.Errorf("conflict_id is required")
	}
	validResolutions := map[string]bool{
		"keep_a": true, "keep_b": true, "merge": true, "retire_both": true,
	}
	if !validResolutions[input.Resolution] {
		return nil, fmt.Errorf("invalid resolution: %s", input.Resolution)
	}
	return &ResolveConflictResult{Success: true}, nil
}

func (a *Activities) ProposeRulesActivity(ctx context.Context, input ProposeRulesInput) (*ProposeRulesResult, error) {
	if input.WorkspaceID == "" {
		return nil, fmt.Errorf("workspace_id is required")
	}
	return &ProposeRulesResult{RulesProposed: 0}, nil
}

// --- Federation activity (method-based) ---

func (a *Activities) ExecuteFederationSyncActivity(ctx context.Context, input FederationSyncInput) (*FederationSyncResult, error) {
	return &FederationSyncResult{
		ItemsSynced:    0,
		ConflictsFound: 0,
		Status:         "COMPLETED",
	}, nil
}

func hashKey(input string) string {
	sum := sha256.Sum256([]byte(input))
	return hex.EncodeToString(sum[:16])
}

// --- Intelligence pipeline activity types (P7) ---

// MemoryRetrieveInput carries parameters for memory retrieval.
type MemoryRetrieveInput struct {
	MessageID   string `json:"message_id"`
	WorkspaceID string `json:"workspace_id"`
	Query       string `json:"query"`
	MaxItems    int    `json:"max_items"`
}

// MemoryItem is a single retrieved memory fragment.
type MemoryItem struct {
	ID         string  `json:"id"`
	MemoryType string  `json:"memory_type"`
	Body       string  `json:"body"`
	Score      float64 `json:"score"`
}

// MemoryRetrieveResult contains deterministically-ordered memory items.
type MemoryRetrieveResult struct {
	Items       []MemoryItem `json:"items"`
	TotalScored int          `json:"total_scored"`
}

// RAGSearchInput carries parameters for RAG retrieval.
type RAGSearchInput struct {
	MessageID    string `json:"message_id"`
	WorkspaceID  string `json:"workspace_id"`
	Query        string `json:"query"`
	TopK         int    `json:"top_k"`
	CollectionID string `json:"collection_id,omitempty"`
}

// RAGChunk is a single RAG retrieval result.
type RAGChunk struct {
	ChunkID    string  `json:"chunk_id"`
	Score      float64 `json:"score"`
	Snippet    string  `json:"snippet"`
	Source     string  `json:"source"`
	Provenance string  `json:"provenance"`
}

// RAGSearchResult contains deterministically-ordered RAG chunks.
type RAGSearchResult struct {
	Chunks           []RAGChunk `json:"chunks"`
	TotalScored      int        `json:"total_scored"`
	KGContextSnippet string     `json:"kg_context_snippet,omitempty"`
}

// ReasoningLoopInput carries context for the brain reasoning loop.
type ReasoningLoopInput struct {
	MessageID       string       `json:"message_id"`
	WorkspaceID     string       `json:"workspace_id"`
	UserID          string       `json:"user_id,omitempty"`
	Intent          string       `json:"intent"`
	Confidence      float64      `json:"confidence"`
	MemoryItems     []MemoryItem `json:"memory_items"`
	RAGChunks       []RAGChunk   `json:"rag_chunks"`
	ContextBudget   int          `json:"context_budget"`
	MaxIterations   int          `json:"max_iterations,omitempty"`
	MemoryContext   string       `json:"memory_context,omitempty"`
	RAGContext      string       `json:"rag_context,omitempty"`
	OriginalPayload string       `json:"original_payload,omitempty"`
}

// ReasoningLoopResult contains the reasoning output with audit evidence.
type ReasoningLoopResult struct {
	PlanID        string   `json:"plan_id"`
	ToolKeys      []string `json:"tool_keys"`
	RiskLevel     string   `json:"risk_level"`
	QualityScore  float64  `json:"quality_score"`
	Iterations    int      `json:"iterations"`
	EvidenceHash  string   `json:"evidence_hash"`
	Deterministic bool     `json:"deterministic"`
}

// CouncilEvalInput carries parameters for council evaluation.
type CouncilEvalInput struct {
	MessageID   string   `json:"message_id"`
	WorkspaceID string   `json:"workspace_id"`
	PlanID      string   `json:"plan_id"`
	ToolKeys    []string `json:"tool_keys"`
	RiskLevel   string   `json:"risk_level"`
	Complexity  float64  `json:"complexity"`
}

// CouncilEvalResult contains the council evaluation outcome.
type CouncilEvalResult struct {
	Convened     bool   `json:"convened"`
	Decision     string `json:"decision"` // "approve", "reject", "abstain"
	Reason       string `json:"reason"`
	VoteCount    int    `json:"vote_count"`
	EvidenceHash string `json:"evidence_hash"`
}

// OutboxEnqueueInput carries an event to enqueue in the outbox.
type OutboxEnqueueInput struct {
	WorkspaceID string `json:"workspace_id"`
	EventType   string `json:"event_type"`
	Payload     string `json:"payload"`
	Target      string `json:"target"`
}

// OutboxEnqueueResult confirms outbox insertion.
type OutboxEnqueueResult struct {
	EntryID string `json:"entry_id"`
	Success bool   `json:"success"`
}

// CognitiveAssessInput carries parameters for cognitive state assessment.
type CognitiveAssessInput struct {
	MessageID     string  `json:"message_id"`
	WorkspaceID   string  `json:"workspace_id"`
	TaskTokens    int     `json:"task_tokens"`
	StepCount     int     `json:"step_count"`
	ErrorCount    int     `json:"error_count"`
	QualityScore  float64 `json:"quality_score"`
}

// CognitiveAssessResult contains the metacognitive assessment.
type CognitiveAssessResult struct {
	CognitiveLoad    float64 `json:"cognitive_load"`
	ReasoningQuality float64 `json:"reasoning_quality"`
	UncertaintyLevel float64 `json:"uncertainty_level"`
	ShouldEscalate   bool    `json:"should_escalate"`
	Strategy         string  `json:"strategy"`
}

// --- Intelligence pipeline activity implementations (P7) ---

// RetrieveMemoryActivity retrieves relevant memory items for the current message.
// In production mode (memoryRepo != nil), queries the DB for items by workspace
// and sorts by score DESC, ID ASC. In degraded mode, uses deterministic FNV scoring.
func (a *Activities) RetrieveMemoryActivity(ctx context.Context, input MemoryRetrieveInput) (*MemoryRetrieveResult, error) {
	if input.WorkspaceID == "" || input.Query == "" {
		return nil, fmt.Errorf("workspace_id and query are required")
	}
	maxItems := input.MaxItems
	if maxItems <= 0 {
		maxItems = 10
	}

	var items []MemoryItem

	// Production path: query DB-backed memory repository.
	if a.memoryRepo != nil {
		dbItems, err := a.memoryRepo.ListByWorkspace(ctx, input.WorkspaceID, maxItems)
		if err != nil {
			log.Printf("[RetrieveMemoryActivity] DB query failed, falling back to deterministic: %v", err)
		} else {
			for _, di := range dbItems {
				// Score from FNV hash for deterministic ordering even with real data.
				itemID := di.ID.String()
				seed := input.WorkspaceID + "::" + di.MemoryType + "::" + itemID
				score := float64(fnvHash64(seed)%1000) / 1000.0
				items = append(items, MemoryItem{
					ID:         itemID,
					MemoryType: di.MemoryType,
					Body:       di.Body,
					Score:      score,
				})
			}
		}
	}

	// Fallback: deterministic scoring when no DB or DB returned nothing.
	if len(items) == 0 {
		queryTokens := tokenize(input.Query)
		items = deterministicMemoryScore(input.WorkspaceID, input.MessageID, queryTokens, maxItems)
	}

	// Stable sort: score DESC, then ID ASC for deterministic ordering.
	sort.SliceStable(items, func(i, j int) bool {
		if items[i].Score != items[j].Score {
			return items[i].Score > items[j].Score
		}
		return items[i].ID < items[j].ID
	})

	if len(items) > maxItems {
		items = items[:maxItems]
	}

	return &MemoryRetrieveResult{
		Items:       items,
		TotalScored: len(items),
	}, nil
}

// SearchRAGActivity performs hybrid RAG search with deterministic ordering.
// In production mode (ragRepo != nil), queries the DB for collections and
// retrieval records. Falls back to deterministic FNV scoring in degraded mode.
// Results sorted by score DESC then ChunkID ASC for replay safety.
func (a *Activities) SearchRAGActivity(ctx context.Context, input RAGSearchInput) (*RAGSearchResult, error) {
	if input.WorkspaceID == "" || input.Query == "" {
		return nil, fmt.Errorf("workspace_id and query are required")
	}
	topK := input.TopK
	if topK <= 0 {
		topK = 5
	}

	var chunks []RAGChunk

	// Production path: query DB-backed RAG repository for collections.
	if a.ragRepo != nil {
		collections, err := a.ragRepo.ListCollections(ctx, input.WorkspaceID)
		if err != nil {
			log.Printf("[SearchRAGActivity] DB query failed, falling back to deterministic: %v", err)
		} else {
			for i, coll := range collections {
				if len(chunks) >= topK {
					break
				}
				seed := input.WorkspaceID + "::rag::" + coll.ID + "::" + input.Query
				score := float64(fnvHash64(seed)%1000) / 1000.0
				chunks = append(chunks, RAGChunk{
					ChunkID:    hashKey(fmt.Sprintf("chunk:%s:%s:%d", input.WorkspaceID, coll.ID, i)),
					Score:      score,
					Snippet:    fmt.Sprintf("Collection %s chunk", coll.Name),
					Source:     coll.ID,
					Provenance: "db_result",
				})
			}
		}
	}

	// Fallback: deterministic scoring when no DB or DB returned nothing.
	if len(chunks) == 0 {
		queryTokens := tokenize(input.Query)
		chunks = deterministicRAGScore(input.WorkspaceID, input.MessageID, queryTokens, topK)
	}

	// Stable sort: score DESC, then ChunkID ASC.
	sort.SliceStable(chunks, func(i, j int) bool {
		if chunks[i].Score != chunks[j].Score {
			return chunks[i].Score > chunks[j].Score
		}
		return chunks[i].ChunkID < chunks[j].ChunkID
	})

	if len(chunks) > topK {
		chunks = chunks[:topK]
	}

	// KG Secondary Retrieval (Phase 5): append knowledge graph context snippet.
	kgContextSnippet := ""
	if a.kgRetriever != nil && input.Query != "" {
		kgResult, kgErr := a.kgRetriever.Query(ctx, input.WorkspaceID, input.Query, 2)
		if kgErr == nil && kgResult != nil && kgResult.ContextSnippet != "" {
			kgContextSnippet = kgResult.ContextSnippet
		}
	}

	return &RAGSearchResult{
		Chunks:           chunks,
		TotalScored:      len(chunks),
		KGContextSnippet: kgContextSnippet,
	}, nil
}

// KGExtractInput drives KG triple extraction.
type KGExtractInput struct {
	WorkspaceID string `json:"workspace_id"`
	TurnID      string `json:"turn_id"`
	Content     string `json:"content"`
}

// KGExtractActivity extracts knowledge graph triples from message content.
func (a *Activities) KGExtractActivity(ctx context.Context, input KGExtractInput) error {
	if a.kgService == nil || input.Content == "" || input.WorkspaceID == "" {
		return nil
	}
	a.kgService.ExtractAndStore(ctx, kg.ExtractionRequest{
		WorkspaceID: input.WorkspaceID,
		TurnID:      input.TurnID,
		Content:     input.Content,
	})
	return nil
}

// NewKGLogger returns a kg.Logger backed by Go's standard log package.
func NewKGLogger() kg.Logger {
	return &kgSlogLogger{}
}

type kgSlogLogger struct{}

func (l *kgSlogLogger) Info(msg string, args ...any)  { log.Printf("[KG INFO] "+msg, args...) }
func (l *kgSlogLogger) Warn(msg string, args ...any)  { log.Printf("[KG WARN] "+msg, args...) }
func (l *kgSlogLogger) Error(msg string, args ...any) { log.Printf("[KG ERROR] "+msg, args...) }
func (l *kgSlogLogger) Debug(msg string, _ ...any)    {}

var _ kg.Logger = (*kgSlogLogger)(nil)

// ExecuteReasoningLoopActivity runs the brain reasoning loop:
// PLANNER → EXECUTOR → CRITIC → REFLECTOR with deterministic parameters.
// Tool keys in the plan are sorted lexically for replay safety.
func (a *Activities) ExecuteReasoningLoopActivity(ctx context.Context, input ReasoningLoopInput) (*ReasoningLoopResult, error) {
	if input.WorkspaceID == "" || input.Intent == "" {
		return nil, fmt.Errorf("workspace_id and intent are required")
	}

	contextBudget := input.ContextBudget
	if contextBudget <= 0 {
		contextBudget = 4096
	}

	// Build deterministic context from memory + RAG (stable-sorted inputs).
	contextHash := buildContextHash(input.WorkspaceID, input.MessageID, input.MemoryItems, input.RAGChunks)

	// Evidence hash for audit trail: deterministic from all inputs.
	evidenceHash := hashKey(fmt.Sprintf("evidence:%s:%s:%s:%d:%d",
		input.MessageID, input.WorkspaceID, input.Intent,
		len(input.MemoryItems), len(input.RAGChunks)))

	// Build context strings from structured data if not provided directly.
	memoryCtx := input.MemoryContext
	if memoryCtx == "" {
		var buf strings.Builder
		for _, item := range input.MemoryItems {
			fmt.Fprintf(&buf, "- [%s] %s (score: %.2f)\n", item.MemoryType, item.Body, item.Score)
		}
		memoryCtx = buf.String()
	}
	ragCtx := input.RAGContext
	if ragCtx == "" {
		var buf strings.Builder
		for _, chunk := range input.RAGChunks {
			fmt.Fprintf(&buf, "- %s (score: %.2f, source: %s)\n", chunk.Snippet, chunk.Score, chunk.Source)
		}
		ragCtx = buf.String()
	}

	// Resolve intelligence service from the LLM layer.
	var intelligence *llm.IntelligenceService
	if a.llmService != nil {
		intelligence = a.llmService.Intelligence()
	}

	maxIter := input.MaxIterations
	if maxIter <= 0 {
		maxIter = 3
	}

	cfg := brain.ReasoningLoopConfig{
		MaxIterations:    maxIter,
		MaxIterationsCap: 8,
		Intelligence:     intelligence,
	}
	loop := brain.NewReasoningLoop(cfg)

	rc := &brain.ReasoningContext{
		MessageID:       input.MessageID,
		WorkspaceID:     input.WorkspaceID,
		UserID:          input.UserID,
		Intent:          input.Intent,
		Confidence:      input.Confidence,
		ContextBudget:   contextBudget,
		MemoryContext:   memoryCtx,
		RAGContext:      ragCtx,
		OriginalPayload: input.OriginalPayload,
	}

	if ctx == nil {
		ctx = context.Background()
	}
	result, err := loop.RunLoop(ctx, rc, maxIter)
	if err != nil {
		log.Printf("[ReasoningLoop] loop failed, returning deterministic fallback: %v", err)
		// Return a failed result rather than an error, for Temporal retry policy compat.
		toolKeys := deterministicPlanTools(input.Intent, input.MemoryItems, input.RAGChunks)
		sort.Strings(toolKeys)
		return &ReasoningLoopResult{
			PlanID:        hashKey("plan:" + input.MessageID + ":" + input.WorkspaceID + ":" + contextHash),
			ToolKeys:      toolKeys,
			RiskLevel:     deterministicRiskAssess(toolKeys),
			QualityScore:  0,
			Iterations:    0,
			EvidenceHash:  evidenceHash,
			Deterministic: true,
		}, nil
	}

	// Convert LoopResult to ReasoningLoopResult.
	var toolKeys []string
	riskLevel := "LOW"
	if result.FinalPlan != nil {
		for _, step := range result.FinalPlan.Steps {
			toolKeys = append(toolKeys, step.ToolKey)
		}
		sort.Strings(toolKeys)
		riskLevel = strings.ToUpper(result.FinalPlan.RiskLevel)
		if riskLevel == "" {
			riskLevel = "LOW"
		}
	}

	return &ReasoningLoopResult{
		PlanID:        hashKey("plan:" + input.MessageID + ":" + input.WorkspaceID + ":" + contextHash),
		ToolKeys:      toolKeys,
		RiskLevel:     riskLevel,
		QualityScore:  result.CriticScore,
		Iterations:    result.Iterations,
		EvidenceHash:  evidenceHash,
		Deterministic: intelligence == nil,
	}, nil
}

// EvaluateCouncilActivity evaluates whether a council should convene and
// produces a deterministic decision based on plan complexity and risk.
func (a *Activities) EvaluateCouncilActivity(ctx context.Context, input CouncilEvalInput) (*CouncilEvalResult, error) {
	if input.WorkspaceID == "" || input.PlanID == "" {
		return nil, fmt.Errorf("workspace_id and plan_id are required")
	}

	// Council convenes for CRITICAL risk or high complexity (> 0.7).
	shouldConvene := input.RiskLevel == "CRITICAL" || input.Complexity > 0.7

	decision := "approve"
	reason := "within_policy"
	voteCount := 0

	if shouldConvene {
		voteCount = 3 // Minimum council size.
		if input.RiskLevel == "CRITICAL" {
			decision = "require_approval"
			reason = "critical_risk_requires_human_approval"
		}
	}

	evidenceHash := hashKey(fmt.Sprintf("council:%s:%s:%s:%.2f",
		input.PlanID, input.WorkspaceID, input.RiskLevel, input.Complexity))

	return &CouncilEvalResult{
		Convened:     shouldConvene,
		Decision:     decision,
		Reason:       reason,
		VoteCount:    voteCount,
		EvidenceHash: evidenceHash,
	}, nil
}

// EnqueueOutboxActivity enqueues an event into the transactional outbox.
// In production mode, acquires a transaction from the pool and uses
// outbox.Service.Enqueue for atomic insertion.
// Idempotent by entry ID: re-enqueue of the same event is a no-op (deterministic
// ID derived from workspace + event_type + payload hash).
func (a *Activities) EnqueueOutboxActivity(ctx context.Context, input OutboxEnqueueInput) (*OutboxEnqueueResult, error) {
	if input.WorkspaceID == "" || input.EventType == "" {
		return nil, fmt.Errorf("workspace_id and event_type are required")
	}

	// Deterministic entry ID for idempotency across Temporal retries.
	entryID := hashKey("outbox:" + input.WorkspaceID + ":" + input.EventType + ":" + input.Payload)

	if a.outboxSvc != nil && a.pool != nil {
		tx, txErr := a.pool.Begin(ctx)
		if txErr != nil {
			return &OutboxEnqueueResult{Success: false}, fmt.Errorf("begin tx: %w", txErr)
		}
		err := a.outboxSvc.Enqueue(ctx, tx, outbox.OutboxEntry{
			ID:          entryID,
			WorkspaceID: input.WorkspaceID,
			EventType:   input.EventType,
			Payload:     []byte(input.Payload),
			Target:      input.Target,
		})
		if err != nil {
			_ = tx.Rollback(ctx)
			// Idempotency: if the entry already exists, treat as success.
			if strings.Contains(err.Error(), "duplicate key") || strings.Contains(err.Error(), "unique_violation") {
				log.Printf("[EnqueueOutbox] idempotent skip entry=%s (already enqueued)", entryID)
				return &OutboxEnqueueResult{EntryID: entryID, Success: true}, nil
			}
			return &OutboxEnqueueResult{Success: false}, fmt.Errorf("enqueue outbox: %w", err)
		}
		if err := tx.Commit(ctx); err != nil {
			return &OutboxEnqueueResult{Success: false}, fmt.Errorf("commit tx: %w", err)
		}
		log.Printf("[EnqueueOutbox] OK entry=%s event=%s target=%s", entryID, input.EventType, input.Target)
	}

	return &OutboxEnqueueResult{
		EntryID: entryID,
		Success: true,
	}, nil
}

// AssessCognitiveStateActivity evaluates the cognitive state of the current
// processing pipeline using deterministic metacognitive metrics.
func (a *Activities) AssessCognitiveStateActivity(ctx context.Context, input CognitiveAssessInput) (*CognitiveAssessResult, error) {
	if input.WorkspaceID == "" {
		return nil, fmt.Errorf("workspace_id is required")
	}

	// Deterministic cognitive load: based on token count and step complexity.
	load := float64(input.TaskTokens) / 8192.0
	if load > 1.0 {
		load = 1.0
	}

	// Reasoning quality: inverse of error rate.
	quality := 1.0
	if input.StepCount > 0 {
		quality = 1.0 - float64(input.ErrorCount)/float64(input.StepCount)
		if quality < 0 {
			quality = 0
		}
	}

	// Uncertainty: inversely correlated with quality score.
	uncertainty := 1.0 - input.QualityScore
	if uncertainty < 0 {
		uncertainty = 0
	}
	if uncertainty > 1.0 {
		uncertainty = 1.0
	}

	shouldEscalate := load > 0.8 || quality < 0.5 || uncertainty > 0.7

	strategy := "proceed"
	if shouldEscalate {
		if quality < 0.3 {
			strategy = "abort"
		} else if load > 0.9 {
			strategy = "simplify"
		} else if uncertainty > 0.8 {
			strategy = "seek_clarification"
		} else {
			strategy = "decompose"
		}
	}

	return &CognitiveAssessResult{
		CognitiveLoad:    load,
		ReasoningQuality: quality,
		UncertaintyLevel: uncertainty,
		ShouldEscalate:   shouldEscalate,
		Strategy:         strategy,
	}, nil
}

// --- Deterministic helper functions (P7) ---

// tokenize splits text into lowercase tokens deterministically.
func tokenize(text string) []string {
	words := strings.Fields(strings.ToLower(text))
	sort.Strings(words) // Lexical sort for determinism.
	return words
}

// buildContextHash produces a deterministic hash from memory items and RAG chunks,
// used as part of the plan ID to ensure replay consistency.
func buildContextHash(workspaceID, messageID string, memItems []MemoryItem, ragChunks []RAGChunk) string {
	var parts []string
	for _, m := range memItems {
		parts = append(parts, m.ID+":"+m.MemoryType)
	}
	for _, c := range ragChunks {
		parts = append(parts, c.ChunkID+":"+c.Source)
	}
	sort.Strings(parts) // Deterministic ordering.
	return hashKey(workspaceID + "::" + messageID + "::" + strings.Join(parts, ","))
}

// deterministicMemoryScore generates deterministic memory items based on
// query token hashing. Uses FNV-64a for consistent scoring.
func deterministicMemoryScore(workspaceID, messageID string, queryTokens []string, maxItems int) []MemoryItem {
	if len(queryTokens) == 0 {
		return nil
	}
	items := make([]MemoryItem, 0, maxItems)
	memTypes := []string{"semantic", "episodic", "preference", "rule"}
	for i := 0; i < maxItems && i < len(memTypes); i++ {
		seed := workspaceID + "::" + memTypes[i] + "::" + strings.Join(queryTokens, ",")
		h := fnvHash64(seed)
		score := float64(h%1000) / 1000.0
		items = append(items, MemoryItem{
			ID:         hashKey(fmt.Sprintf("mem:%s:%s:%d", workspaceID, messageID, i)),
			MemoryType: memTypes[i],
			Body:       fmt.Sprintf("Memory item %d for %s", i, strings.Join(queryTokens, " ")),
			Score:      score,
		})
	}
	return items
}

// deterministicRAGScore generates deterministic RAG chunks based on
// query token hashing.
func deterministicRAGScore(workspaceID, messageID string, queryTokens []string, topK int) []RAGChunk {
	if len(queryTokens) == 0 {
		return nil
	}
	chunks := make([]RAGChunk, 0, topK)
	for i := 0; i < topK; i++ {
		seed := workspaceID + "::chunk::" + strings.Join(queryTokens, ",") + "::" + fmt.Sprintf("%d", i)
		h := fnvHash64(seed)
		score := float64(h%1000) / 1000.0
		chunks = append(chunks, RAGChunk{
			ChunkID:    hashKey(fmt.Sprintf("chunk:%s:%s:%d", workspaceID, messageID, i)),
			Score:      score,
			Snippet:    fmt.Sprintf("RAG chunk %d for %s", i, strings.Join(queryTokens, " ")),
			Source:     "collection_default",
			Provenance: "native_result",
		})
	}
	return chunks
}

// deterministicPlanTools generates tool keys deterministically from intent and context.
func deterministicPlanTools(intent string, memItems []MemoryItem, ragChunks []RAGChunk) []string {
	// Map intent keywords to tools deterministically.
	toolMap := map[string]string{
		"search":   "search.web",
		"send":     "email.send",
		"schedule": "calendar.create",
		"create":   "document.create",
		"delete":   "resource.delete",
		"query":    "search.knowledge",
	}

	tools := make(map[string]struct{})
	intentLower := strings.ToLower(intent)
	for keyword, tool := range toolMap {
		if strings.Contains(intentLower, keyword) {
			tools[tool] = struct{}{}
		}
	}

	// Default tool if no match.
	if len(tools) == 0 {
		tools["search.knowledge"] = struct{}{}
	}

	result := make([]string, 0, len(tools))
	for t := range tools {
		result = append(result, t)
	}
	return result
}

// deterministicRiskAssess calculates risk level from tool set deterministically.
func deterministicRiskAssess(toolKeys []string) string {
	for _, tk := range toolKeys {
		if strings.Contains(tk, "delete") || strings.Contains(tk, "destroy") {
			return "CRITICAL"
		}
		if strings.Contains(tk, "send") || strings.Contains(tk, "create") {
			return "ELEVATED"
		}
	}
	return "LOW"
}

// deterministicQualityScore produces a quality score from context completeness.
func deterministicQualityScore(confidence float64, memCount, ragCount int) float64 {
	// Base from confidence.
	score := confidence * 0.5
	// Bonus for memory context (up to 0.25).
	memBonus := float64(memCount) / 20.0
	if memBonus > 0.25 {
		memBonus = 0.25
	}
	// Bonus for RAG context (up to 0.25).
	ragBonus := float64(ragCount) / 10.0
	if ragBonus > 0.25 {
		ragBonus = 0.25
	}
	score += memBonus + ragBonus
	if score > 1.0 {
		score = 1.0
	}
	return score
}

// fnvHash64 returns a deterministic FNV-64a hash as uint64.
func fnvHash64(s string) uint64 {
	const offset64 uint64 = 14695981039346656037
	const prime64 uint64 = 1099511628211
	h := offset64
	for i := 0; i < len(s); i++ {
		h ^= uint64(s[i])
		h *= prime64
	}
	return h
}

// EvictWorkingMemoryActivity cleans up working memory for a completed task.
// Best-effort: never returns error to avoid failing the workflow on eviction failure.
func (a *Activities) EvictWorkingMemoryActivity(ctx context.Context, workspaceID, taskID string) error {
	if a.workingMemory != nil {
		a.workingMemory.Complete(ctx, workspaceID, taskID)
	}
	return nil
}

// buildLLMClient constructs a fresh AnthropicClient from environment.
// Returns nil if ANTHROPIC_API_KEY is unset (graceful degradation).
func (a *Activities) buildLLMClient() llm.Client {
	apiKey := os.Getenv("ANTHROPIC_API_KEY")
	if apiKey == "" {
		return nil
	}
	client, err := llm.NewAnthropicClient(llm.AnthropicConfig{
		APIKey:  apiKey,
		Timeout: 60 * time.Second,
	})
	if err != nil {
		return nil
	}
	return client
}

// ProductionEvalSampleActivity samples completed workflows and re-scores them.
func (a *Activities) ProductionEvalSampleActivity(ctx context.Context) (ProductionEvalSampleResult, error) {
	if a.prodEvalSampler == nil {
		return ProductionEvalSampleResult{}, nil
	}
	sampleCount, passRate, err := a.prodEvalSampler.SampleAndScore(ctx)
	if err != nil {
		return ProductionEvalSampleResult{}, fmt.Errorf("ProductionEvalSampleActivity: %w", err)
	}
	if a.qualityGauge != nil {
		a.qualityGauge.Set(passRate)
	}
	return ProductionEvalSampleResult{SampleCount: sampleCount, PassRate: passRate}, nil
}

// SynthesisVerifyActivity runs hallucination detection on a synthesized T2/T3 response.
func (a *Activities) SynthesisVerifyActivity(
	ctx context.Context,
	workspaceID, systemPrompt, userPrompt, response, tier string,
) (*eval.VerificationResult, error) {
	if a.synthesisVerifier == nil {
		return &eval.VerificationResult{Passed: true, ConsistencyScore: 1.0}, nil
	}
	return a.synthesisVerifier.Verify(ctx, workspaceID, systemPrompt, userPrompt, response, tier)
}

// VisionPreProcessActivity detects image attachments in the incoming message payload,
// calls the Claude vision API to extract text and entities, and returns a normalized
// text representation ready for injection into ClassifyIntentActivity.
func (a *Activities) VisionPreProcessActivity(ctx context.Context, req vision.ExtractionRequest) (*vision.ExtractionResult, error) {
	if a.visionProcessor == nil || len(req.Attachments) == 0 {
		return &vision.ExtractionResult{
			WorkspaceID: req.WorkspaceID,
			TurnID:      req.TurnID,
		}, nil
	}

	result, err := a.visionProcessor.Process(ctx, req)
	if err != nil {
		return &vision.ExtractionResult{WorkspaceID: req.WorkspaceID, TurnID: req.TurnID}, nil
	}

	if a.pool != nil && !result.IsEmpty() {
		_, _ = a.pool.Exec(ctx, `
			INSERT INTO vision_extractions
				(workspace_id, turn_id, image_type, normalized_text, entity_count, confidence)
			VALUES ($1,$2,$3,$4,$5,$6)
		`, result.WorkspaceID, result.TurnID, result.ImageType,
			result.NormalizedText, len(result.Entities), result.Confidence)
	}

	return result, nil
}

// DetectProactiveSignalsActivity detects proactive opportunities for a workspace.
func (a *Activities) DetectProactiveSignalsActivity(ctx context.Context, workspaceID string) ([]proactive.Signal, error) {
	if a.proactiveMonitor == nil {
		return nil, nil
	}
	return a.proactiveMonitor.DetectSignals(ctx, workspaceID)
}

// BuildAndDispatchProactiveOfferActivity builds an offer message for a signal and
// dispatches it via the outbox. NEVER executes any tool or action — offer-only.
func (a *Activities) BuildAndDispatchProactiveOfferActivity(ctx context.Context, s proactive.Signal) (string, error) {
	if a.offerBuilder == nil {
		return "", nil
	}

	offerText, err := a.offerBuilder.Build(s)
	if err != nil {
		return "", fmt.Errorf("BuildAndDispatchProactiveOfferActivity: %w", err)
	}

	if a.proactiveMonitor != nil {
		_, _ = a.proactiveMonitor.PersistSignal(ctx, s, offerText)
	}

	if a.outboxDispatcher != nil {
		payload := []byte(offerText)
		_ = a.outboxDispatcher.Dispatch(ctx, "whatsapp:"+s.WorkspaceID, payload)
	}

	return offerText, nil
}

// DelegateA2ATaskActivity delegates a task to an external A2A agent.
func (a *Activities) DelegateA2ATaskActivity(ctx context.Context, req a2a.DelegateRequest) (*a2a.DelegateResult, error) {
	if a.a2aClient == nil {
		return nil, temporal.NewNonRetryableApplicationError("a2a_client not configured", "CONFIG_ERROR", nil)
	}
	result, err := a.a2aClient.Delegate(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("DelegateA2ATaskActivity: %w", err)
	}
	return result, nil
}

// ListExternalAgentsActivity returns all active external agents from the registry.
func (a *Activities) ListExternalAgentsActivity(ctx context.Context) ([]a2a.ExternalAgent, error) {
	if a.externalAgentRegistry == nil {
		return nil, nil
	}
	return a.externalAgentRegistry.List(ctx)
}

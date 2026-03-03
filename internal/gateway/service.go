package gateway

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

var (
	ErrInvalidSignature = errors.New("invalid signature")
	ErrReplayDetected   = errors.New("replay detected")
	ErrRateLimited      = errors.New("rate limited")
)

type AttachmentInput struct {
	URL       string `json:"url"`
	MIMEType  string `json:"mime_type"`
	Filename  string `json:"filename"`
	SizeBytes int64  `json:"size_bytes"`
}

type AttachmentReference struct {
	ID        string
	SourceURL string
	S3URI     string
	MIMEType  string
	Filename  string
	SizeBytes int64
}

type IngressTurn struct {
	ID                     uuid.UUID
	WorkspaceID            uuid.UUID
	UserChannelID          string
	DedupHash              string
	Payload                []byte
	ParsedInteractiveReply string
	ParsedDiscoveryAnswer  string
	Transcript             string
	Attachments            []AttachmentReference
	CreatedAt              time.Time
}

type ChannelIdentityEnvelope struct {
	IngressTurnID      uuid.UUID
	ChannelType        string
	ClaimedIdentifier  string
	VerificationMethod string
	VerificationResult string
	RawSignature       string
	VerifiedAt         time.Time
}

type QueueMessage struct {
	IngressTurnID uuid.UUID
	WorkspaceID   uuid.UUID
	GroupKey      string
	DedupKey      string
	Payload       []byte
}

type OutboundDispatch struct {
	ID                uuid.UUID
	WorkspaceID       uuid.UUID
	Channel           string
	ChannelIdentifier string
	Body              string
	QueuedAt          time.Time
}

type InteractiveIntent string

const (
	IntentApprove InteractiveIntent = "APPROVE"
	IntentDeny    InteractiveIntent = "DENY"
	IntentUndo    InteractiveIntent = "UNDO"
	IntentEdit    InteractiveIntent = "EDIT"
	IntentOption  InteractiveIntent = "OPTION"
	IntentNone    InteractiveIntent = ""
)

type InMemoryStore struct {
	mu          sync.Mutex
	byDedupHash map[string]IngressTurn
	byNonce     map[string]time.Time
	turns       []IngressTurn
}

func NewInMemoryStore() *InMemoryStore {
	return &InMemoryStore{
		byDedupHash: map[string]IngressTurn{},
		byNonce:     map[string]time.Time{},
		turns:       []IngressTurn{},
	}
}

func (s *InMemoryStore) AddNonce(nonce string, ttl time.Duration) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now().UTC()
	for key, expiresAt := range s.byNonce {
		if now.After(expiresAt) {
			delete(s.byNonce, key)
		}
	}
	if _, exists := s.byNonce[nonce]; exists {
		return false
	}
	s.byNonce[nonce] = now.Add(ttl)
	return true
}

func (s *InMemoryStore) InsertIngressTurn(turn IngressTurn) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.byDedupHash[turn.DedupHash]; exists {
		return false
	}
	s.byDedupHash[turn.DedupHash] = turn
	s.turns = append(s.turns, turn)
	return true
}

func (s *InMemoryStore) TurnCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.turns)
}

func (s *InMemoryStore) LastTurn() (IngressTurn, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.turns) == 0 {
		return IngressTurn{}, false
	}
	return s.turns[len(s.turns)-1], true
}

type InMemoryQueue struct {
	mu       sync.Mutex
	messages []QueueMessage
}

func (q *InMemoryQueue) Enqueue(msg QueueMessage) {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.messages = append(q.messages, msg)
}

func (q *InMemoryQueue) Count() int {
	q.mu.Lock()
	defer q.mu.Unlock()
	return len(q.messages)
}

func (q *InMemoryQueue) Pop() (QueueMessage, bool) {
	q.mu.Lock()
	defer q.mu.Unlock()
	if len(q.messages) == 0 {
		return QueueMessage{}, false
	}
	msg := q.messages[0]
	q.messages = q.messages[1:]
	return msg, true
}

type RateLimiter struct {
	mu          sync.Mutex
	hourlyHits  map[string][]time.Time
	minuteHits  map[string][]time.Time
	nowFn       func() time.Time
	hourlyLimit map[string]int
	minuteLimit map[string]int
}

func NewRateLimiter() *RateLimiter {
	return &RateLimiter{
		hourlyHits: map[string][]time.Time{},
		minuteHits: map[string][]time.Time{},
		nowFn:      time.Now,
		hourlyLimit: map[string]int{
			"free":       30,
			"pro":        120,
			"enterprise": 1_000_000,
			"admin":      1_000_000,
			"service":    1_000_000,
		},
		minuteLimit: map[string]int{
			"free":       30,
			"pro":        60,
			"enterprise": 1_000_000,
			"admin":      1_000_000,
			"service":    1_000_000,
		},
	}
}

func (r *RateLimiter) SetNowForTest(nowFn func() time.Time) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if nowFn != nil {
		r.nowFn = nowFn
	}
}

func (r *RateLimiter) Allow(subject, tier string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	tier = normalizeTier(tier)
	if tier == "enterprise" || tier == "admin" || tier == "service" {
		return true
	}

	now := r.nowFn().UTC()
	hourlyWindowStart := now.Add(-1 * time.Hour)
	minuteWindowStart := now.Add(-1 * time.Minute)

	hourly := pruneBefore(r.hourlyHits[subject], hourlyWindowStart)
	minutely := pruneBefore(r.minuteHits[subject], minuteWindowStart)

	if len(hourly) >= r.hourlyLimit[tier] || len(minutely) >= r.minuteLimit[tier] {
		r.hourlyHits[subject] = hourly
		r.minuteHits[subject] = minutely
		return false
	}

	hourly = append(hourly, now)
	minutely = append(minutely, now)
	r.hourlyHits[subject] = hourly
	r.minuteHits[subject] = minutely
	return true
}

func pruneBefore(hits []time.Time, threshold time.Time) []time.Time {
	if len(hits) == 0 {
		return hits
	}
	out := make([]time.Time, 0, len(hits))
	for _, hit := range hits {
		if !hit.Before(threshold) {
			out = append(out, hit)
		}
	}
	return out
}

func normalizeTier(tier string) string {
	switch strings.ToLower(strings.TrimSpace(tier)) {
	case "free", "pro", "enterprise", "admin", "service":
		return strings.ToLower(strings.TrimSpace(tier))
	default:
		return "pro"
	}
}

type AuditLog struct {
	mu      sync.Mutex
	entries []string
}

func (a *AuditLog) Append(event string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.entries = append(a.entries, event)
}

func (a *AuditLog) Count() int {
	a.mu.Lock()
	defer a.mu.Unlock()
	return len(a.entries)
}

func (a *AuditLog) Entries() []string {
	a.mu.Lock()
	defer a.mu.Unlock()
	out := make([]string, len(a.entries))
	copy(out, a.entries)
	return out
}

type cachedHTTPResponse struct {
	statusCode int
	body       []byte
	expiresAt  time.Time
}

type IdempotencyStore struct {
	mu    sync.Mutex
	ttl   time.Duration
	nowFn func() time.Time
	byKey map[string]cachedHTTPResponse
}

func NewIdempotencyStore(ttl time.Duration) *IdempotencyStore {
	if ttl <= 0 {
		ttl = 24 * time.Hour
	}
	return &IdempotencyStore{
		ttl:   ttl,
		nowFn: time.Now,
		byKey: map[string]cachedHTTPResponse{},
	}
}

func (s *IdempotencyStore) SetNowForTest(nowFn func() time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if nowFn != nil {
		s.nowFn = nowFn
	}
}

func (s *IdempotencyStore) Get(key string) (statusCode int, body []byte, ok bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cleanupLocked()
	entry, found := s.byKey[key]
	if !found {
		return 0, nil, false
	}
	out := make([]byte, len(entry.body))
	copy(out, entry.body)
	return entry.statusCode, out, true
}

func (s *IdempotencyStore) Set(key string, statusCode int, body []byte) {
	s.mu.Lock()
	defer s.mu.Unlock()
	buf := make([]byte, len(body))
	copy(buf, body)
	s.byKey[key] = cachedHTTPResponse{
		statusCode: statusCode,
		body:       buf,
		expiresAt:  s.nowFn().UTC().Add(s.ttl),
	}
}

func (s *IdempotencyStore) cleanupLocked() {
	now := s.nowFn().UTC()
	for key, value := range s.byKey {
		if now.After(value.expiresAt) {
			delete(s.byKey, key)
		}
	}
}

type WorkspaceRouter struct {
	mu                     sync.RWMutex
	bindings               map[string]uuid.UUID
	defaultWorkspaceByUser map[string]uuid.UUID
}

func NewWorkspaceRouter() *WorkspaceRouter {
	return &WorkspaceRouter{
		bindings:               map[string]uuid.UUID{},
		defaultWorkspaceByUser: map[string]uuid.UUID{},
	}
}

func (r *WorkspaceRouter) Bind(channel, identifier string, workspaceID uuid.UUID) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.bindings[channel+"::"+identifier] = workspaceID
}

func (r *WorkspaceRouter) Resolve(channel, identifier string) (uuid.UUID, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	workspaceID, ok := r.bindings[channel+"::"+identifier]
	if !ok {
		return uuid.Nil, fmt.Errorf("workspace binding not found")
	}
	return workspaceID, nil
}

func (r *WorkspaceRouter) SetUserDefaultWorkspace(channel, identifier string, workspaceID uuid.UUID) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.defaultWorkspaceByUser[channel+"::"+identifier] = workspaceID
}

// ResolveForInbound implements addendum routing behavior with fallback auto-binding:
// 1) existing channel binding
// 2) fallback to user default workspace and auto-bind
// 3) otherwise unbound error
func (r *WorkspaceRouter) ResolveForInbound(channel, identifier string) (workspaceID uuid.UUID, autoBound bool, err error) {
	key := channel + "::" + identifier
	r.mu.RLock()
	if boundWorkspace, ok := r.bindings[key]; ok {
		r.mu.RUnlock()
		return boundWorkspace, false, nil
	}
	defaultWorkspace, hasDefault := r.defaultWorkspaceByUser[key]
	r.mu.RUnlock()

	if !hasDefault {
		return uuid.Nil, false, fmt.Errorf("workspace binding not found")
	}

	r.mu.Lock()
	r.bindings[key] = defaultWorkspace
	r.mu.Unlock()
	return defaultWorkspace, true, nil
}

type AttachmentUploader interface {
	Upload(ctx context.Context, workspaceID uuid.UUID, attachment AttachmentInput) (AttachmentReference, error)
}

type InMemoryAttachmentUploader struct {
	bucket string
}

func NewInMemoryAttachmentUploader(bucket string) *InMemoryAttachmentUploader {
	if strings.TrimSpace(bucket) == "" {
		bucket = "attachments"
	}
	return &InMemoryAttachmentUploader{bucket: bucket}
}

func (u *InMemoryAttachmentUploader) Upload(_ context.Context, workspaceID uuid.UUID, attachment AttachmentInput) (AttachmentReference, error) {
	if strings.TrimSpace(attachment.URL) == "" {
		return AttachmentReference{}, fmt.Errorf("attachment url is required")
	}
	attachmentID, err := uuid.NewV7()
	if err != nil {
		return AttachmentReference{}, err
	}
	filename := strings.TrimSpace(attachment.Filename)
	if filename == "" {
		filename = path.Base(attachment.URL)
		if filename == "." || filename == "/" {
			filename = "attachment"
		}
	}
	return AttachmentReference{
		ID:        attachmentID.String(),
		SourceURL: attachment.URL,
		S3URI:     fmt.Sprintf("s3://%s/%s/%s-%s", u.bucket, workspaceID.String(), attachmentID.String(), filename),
		MIMEType:  attachment.MIMEType,
		Filename:  filename,
		SizeBytes: attachment.SizeBytes,
	}, nil
}

type Service struct {
	secret      []byte
	nonceTTL    time.Duration
	idempTTL    time.Duration
	startedAt   time.Time
	store       *InMemoryStore
	queue       *InMemoryQueue
	rateLimiter *RateLimiter
	idempotency *IdempotencyStore
	audit       *AuditLog
	identityMu  sync.Mutex
	identityLog []ChannelIdentityEnvelope
	router      *WorkspaceRouter
	uploader    AttachmentUploader

	injectedMu  sync.Mutex
	injectedCnt int

	outboxMu sync.Mutex
	outbox   []OutboundDispatch
}

func NewService(secret string) *Service {
	idempotencyTTL := 24 * time.Hour
	return &Service{
		secret:      []byte(secret),
		nonceTTL:    10 * time.Minute,
		idempTTL:    idempotencyTTL,
		startedAt:   time.Now().UTC(),
		store:       NewInMemoryStore(),
		queue:       &InMemoryQueue{},
		rateLimiter: NewRateLimiter(),
		idempotency: NewIdempotencyStore(idempotencyTTL),
		audit:       &AuditLog{},
		identityLog: []ChannelIdentityEnvelope{},
		router:      NewWorkspaceRouter(),
		uploader:    NewInMemoryAttachmentUploader("attachments"),
		outbox:      []OutboundDispatch{},
	}
}

type inboundMessage struct {
	Channel           string            `json:"channel"`
	ChannelIdentifier string            `json:"channel_identifier"`
	UserChannelID     string            `json:"user_channel_id"`
	ChannelMessageID  string            `json:"channel_message_id"`
	UserTier          string            `json:"user_tier"`
	Nonce             string            `json:"nonce"`
	Message           string            `json:"message"`
	InteractiveReply  string            `json:"interactive_reply"`
	DiscoveryAnswer   string            `json:"discovery_answer"`
	AudioURL          string            `json:"audio_url"`
	Attachments       []AttachmentInput `json:"attachments"`
}

type outboundSendRequest struct {
	WorkspaceID       string `json:"workspace_id"`
	Channel           string `json:"channel"`
	ChannelIdentifier string `json:"channel_identifier"`
	Body              string `json:"body"`
}

type injectedToolCall struct {
	WorkspaceID   string         `json:"workspace_id"`
	IngressTurnID string         `json:"ingress_turn_id"`
	ToolKey       string         `json:"tool_key"`
	Arguments     map[string]any `json:"arguments"`
}

func signatureFor(secret, payload []byte) string {
	mac := hmac.New(sha256.New, secret)
	mac.Write(payload)
	return hex.EncodeToString(mac.Sum(nil))
}

func normalizeSignature(signature string) string {
	signature = strings.TrimSpace(strings.ToLower(signature))
	signature = strings.TrimPrefix(signature, "sha256=")
	return signature
}

func (s *Service) validateSignature(payload []byte, signature string) error {
	signature = normalizeSignature(signature)
	expected := signatureFor(s.secret, payload)
	if signature == "" || !hmac.Equal([]byte(expected), []byte(signature)) {
		s.audit.Append("BREVIO.security.webhook.signature_invalid.v1")
		return ErrInvalidSignature
	}
	return nil
}

func dedupHash(payload []byte) string {
	sum := sha256.Sum256(payload)
	return hex.EncodeToString(sum[:])
}

func gatewayIdempotencyKey(channel, channelMessageID string) string {
	channelMessageID = strings.TrimSpace(channelMessageID)
	if channelMessageID == "" {
		return ""
	}
	channel = strings.ToLower(strings.TrimSpace(channel))
	if channel == "" {
		channel = "unknown"
	}
	return channel + "::" + channelMessageID
}

func (s *Service) HandleInbound(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	signature := r.Header.Get("X-Signature")
	if signature == "" {
		signature = r.Header.Get("X-Hub-Signature-256")
	}
	if err := s.validateSignature(body, signature); err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}

	var msg inboundMessage
	if err := json.Unmarshal(body, &msg); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if msg.Nonce == "" {
		http.Error(w, "nonce is required", http.StatusBadRequest)
		return
	}
	if msg.UserChannelID == "" {
		http.Error(w, "user_channel_id is required", http.StatusBadRequest)
		return
	}

	idempotencyKey := gatewayIdempotencyKey(msg.Channel, msg.ChannelMessageID)
	if idempotencyKey != "" {
		if statusCode, cachedBody, ok := s.idempotency.Get(idempotencyKey); ok {
			s.audit.Append("BREVIO.ingress.idempotent_replay.v1")
			w.WriteHeader(statusCode)
			_, _ = w.Write(cachedBody)
			return
		}
	}

	if !s.store.AddNonce(msg.Nonce, s.nonceTTL) {
		s.audit.Append("BREVIO.security.webhook.replay_blocked.v1")
		http.Error(w, ErrReplayDetected.Error(), http.StatusConflict)
		return
	}

	if !s.rateLimiter.Allow(msg.Channel+"::"+msg.UserChannelID, msg.UserTier) {
		s.audit.Append("BREVIO.gateway.rate_limited.v1")
		http.Error(w, ErrRateLimited.Error(), http.StatusTooManyRequests)
		return
	}

	workspaceID, _, err := s.router.ResolveForInbound(msg.Channel, msg.ChannelIdentifier)
	if err != nil {
		s.audit.Append("BREVIO.security.identity.verification_failed.v1")
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	turnID, err := uuid.NewV7()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	attachmentRefs := make([]AttachmentReference, 0, len(msg.Attachments))
	for _, attachment := range msg.Attachments {
		if err := ValidateAttachmentInput(attachment); err != nil {
			s.audit.Append("BREVIO.ingress.attachment_rejected.v1")
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		ref, uploadErr := s.uploader.Upload(r.Context(), workspaceID, attachment)
		if uploadErr != nil {
			http.Error(w, uploadErr.Error(), http.StatusBadGateway)
			return
		}
		attachmentRefs = append(attachmentRefs, ref)
	}

	turn := IngressTurn{
		ID:                     turnID,
		WorkspaceID:            workspaceID,
		UserChannelID:          msg.UserChannelID,
		DedupHash:              dedupHash(body),
		Payload:                body,
		ParsedInteractiveReply: s.ParseInteractiveReply(msg.InteractiveReply),
		ParsedDiscoveryAnswer:  s.ParseDiscoveryAnswer(msg.DiscoveryAnswer),
		Transcript:             s.PreprocessVoice(r.Context(), msg.AudioURL),
		Attachments:            attachmentRefs,
		CreatedAt:              time.Now().UTC(),
	}
	s.recordIdentityEnvelope(ChannelIdentityEnvelope{
		IngressTurnID:      turn.ID,
		ChannelType:        msg.Channel,
		ClaimedIdentifier:  msg.ChannelIdentifier,
		VerificationMethod: "webhook_signature+sender_binding",
		VerificationResult: "verified",
		RawSignature:       signature,
		VerifiedAt:         time.Now().UTC(),
	})

	if inserted := s.store.InsertIngressTurn(turn); !inserted {
		s.audit.Append("BREVIO.ingress.duplicate_dropped.v1")
		if idempotencyKey != "" {
			s.idempotency.Set(idempotencyKey, http.StatusOK, []byte(`{"status":"duplicate_dropped"}`))
		}
		w.WriteHeader(http.StatusOK)
		return
	}
	s.audit.Append("BREVIO.ingress.received.v1")

	s.queue.Enqueue(QueueMessage{
		IngressTurnID: turn.ID,
		WorkspaceID:   turn.WorkspaceID,
		GroupKey:      msg.UserChannelID,
		DedupKey:      turn.ID.String(),
		Payload:       body,
	})

	acceptedBody := []byte(`{"status":"accepted"}`)
	if idempotencyKey != "" {
		s.idempotency.Set(idempotencyKey, http.StatusAccepted, acceptedBody)
	}
	w.WriteHeader(http.StatusAccepted)
	_, _ = w.Write(acceptedBody)
}

func (s *Service) recordIdentityEnvelope(envelope ChannelIdentityEnvelope) {
	s.identityMu.Lock()
	defer s.identityMu.Unlock()
	s.identityLog = append(s.identityLog, envelope)
}

func (s *Service) ChannelIdentityEnvelopeCount() int {
	s.identityMu.Lock()
	defer s.identityMu.Unlock()
	return len(s.identityLog)
}

func (s *Service) HandleInjectToolCall(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	var call injectedToolCall
	if err := json.Unmarshal(body, &call); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if call.ToolKey == "" {
		http.Error(w, "tool_key is required", http.StatusBadRequest)
		return
	}

	s.injectedMu.Lock()
	s.injectedCnt++
	s.injectedMu.Unlock()

	w.WriteHeader(http.StatusAccepted)
	_, _ = w.Write([]byte(`{"status":"accepted"}`))
}

func (s *Service) InjectedToolCallCount() int {
	s.injectedMu.Lock()
	defer s.injectedMu.Unlock()
	return s.injectedCnt
}

func (s *Service) BindWorkspace(channel, identifier string, workspaceID uuid.UUID) {
	s.router.Bind(channel, identifier, workspaceID)
}

func (s *Service) QueueMessageCount() int {
	return s.queue.Count()
}

func (s *Service) IngressTurnCount() int {
	return s.store.TurnCount()
}

func (s *Service) LastIngressTurn() (IngressTurn, bool) {
	return s.store.LastTurn()
}

func (s *Service) AuditEntries() []string {
	return s.audit.Entries()
}

func (s *Service) PopQueueMessage() (QueueMessage, bool) {
	return s.queue.Pop()
}

func (s *Service) SetAttachmentUploader(uploader AttachmentUploader) {
	if uploader == nil {
		return
	}
	s.uploader = uploader
}

func (s *Service) HandleWhatsAppVerification(w http.ResponseWriter, r *http.Request) {
	challenge := r.URL.Query().Get("hub.challenge")
	if challenge == "" {
		http.Error(w, "missing challenge", http.StatusBadRequest)
		return
	}
	_, _ = w.Write([]byte(challenge))
}

func (s *Service) HandleOutboundSend(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	var outbound outboundSendRequest
	if err := json.Unmarshal(body, &outbound); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if outbound.WorkspaceID == "" || outbound.Channel == "" || outbound.ChannelIdentifier == "" || outbound.Body == "" {
		http.Error(w, "workspace_id, channel, channel_identifier, and body are required", http.StatusBadRequest)
		return
	}
	workspaceID, err := uuid.Parse(outbound.WorkspaceID)
	if err != nil {
		http.Error(w, "workspace_id must be a valid uuid", http.StatusBadRequest)
		return
	}
	outboundID, err := uuid.NewV7()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	s.outboxMu.Lock()
	s.outbox = append(s.outbox, OutboundDispatch{
		ID:                outboundID,
		WorkspaceID:       workspaceID,
		Channel:           outbound.Channel,
		ChannelIdentifier: outbound.ChannelIdentifier,
		Body:              outbound.Body,
		QueuedAt:          time.Now().UTC(),
	})
	s.outboxMu.Unlock()
	w.WriteHeader(http.StatusAccepted)
	_, _ = w.Write([]byte(`{"status":"queued"}`))
}

func (s *Service) OutboundDispatchCount() int {
	s.outboxMu.Lock()
	defer s.outboxMu.Unlock()
	return len(s.outbox)
}

func (s *Service) HandleHealth(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "/health" || r.URL.Path == "/health/deep" {
		checks := map[string]string{
			"process": "ok",
		}
		if r.URL.Path == "/health/deep" {
			checks["db"] = envCheck("DATABASE_URL")
			checks["redis"] = envCheck("REDIS_URL")
			checks["temporal"] = envCheck("TEMPORAL_HOST")
		}
		version := strings.TrimSpace(os.Getenv("SERVICE_VERSION"))
		if version == "" {
			version = "0.1.0"
		}
		payload := map[string]any{
			"status":    "healthy",
			"version":   version,
			"uptime_ms": time.Since(s.startedAt).Milliseconds(),
			"checks":    checks,
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(payload)
		return
	}

	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ok"))
}

func envCheck(key string) string {
	if strings.TrimSpace(os.Getenv(key)) == "" {
		return "not_configured"
	}
	return "configured"
}

func (s *Service) ParseInteractiveReply(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return ""
	}

	plain := parseIntentFromText(trimmed)
	if plain != "" {
		return plain
	}

	var payload map[string]any
	if err := json.Unmarshal([]byte(trimmed), &payload); err != nil {
		return trimmed
	}
	if button, ok := payload["button_reply"].(map[string]any); ok {
		id, _ := button["id"].(string)
		title, _ := button["title"].(string)
		switch strings.ToLower(strings.TrimSpace(id)) {
		case "approve":
			return string(IntentApprove)
		case "deny":
			return string(IntentDeny)
		}
		if intent := parseIntentFromText(id + " " + title); intent != "" {
			return intent
		}
		if strings.TrimSpace(id) != "" {
			return "OPTION:" + strings.TrimSpace(id)
		}
		return trimmed
	}
	if list, ok := payload["list_reply"].(map[string]any); ok {
		id, _ := list["id"].(string)
		title, _ := list["title"].(string)
		switch strings.ToLower(strings.TrimSpace(id)) {
		case "approve":
			return string(IntentApprove)
		case "deny":
			return string(IntentDeny)
		}
		if intent := parseIntentFromText(id + " " + title); intent != "" {
			return intent
		}
		if strings.TrimSpace(id) != "" {
			return "OPTION:" + strings.TrimSpace(id)
		}
		return trimmed
	}
	return trimmed
}

func (s *Service) ParseDiscoveryAnswer(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return ""
	}
	var payload map[string]any
	if err := json.Unmarshal([]byte(trimmed), &payload); err != nil {
		return trimmed
	}
	question, _ := payload["question"].(string)
	answer, _ := payload["answer"].(string)
	if question != "" || answer != "" {
		return strings.TrimSpace("question=" + question + ";answer=" + answer)
	}
	return trimmed
}

func (s *Service) PreprocessVoice(_ context.Context, audioURL string) string {
	audioURL = strings.TrimSpace(audioURL)
	if audioURL == "" {
		return ""
	}
	return "transcript:" + audioURL
}

func parseIntentFromText(raw string) string {
	normalized := strings.ToLower(strings.TrimSpace(raw))
	if normalized == "" {
		return ""
	}

	if regexp.MustCompile(`^(yes|y|approve|confirm|ok|go ahead|do it)$`).MatchString(normalized) {
		return string(IntentApprove)
	}
	if regexp.MustCompile(`^(no|n|deny|cancel|stop|don't|dont|reject)$`).MatchString(normalized) {
		return string(IntentDeny)
	}
	if regexp.MustCompile(`^(undo|revert|take it back|rollback)$`).MatchString(normalized) {
		return string(IntentUndo)
	}
	if strings.HasPrefix(normalized, "edit ") || strings.HasPrefix(normalized, "change ") || strings.HasPrefix(normalized, "modify ") {
		parts := strings.SplitN(raw, " ", 2)
		if len(parts) == 2 {
			return string(IntentEdit) + ":" + strings.TrimSpace(parts[1])
		}
		return string(IntentEdit)
	}
	if index, err := strconv.Atoi(normalized); err == nil && index > 0 {
		return "OPTION_INDEX:" + strconv.Itoa(index)
	}
	return ""
}

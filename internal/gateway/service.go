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
	mu      sync.Mutex
	counts  map[string]int
	maxHits int
}

func NewRateLimiter(maxHits int) *RateLimiter {
	return &RateLimiter{counts: map[string]int{}, maxHits: maxHits}
}

func (r *RateLimiter) Allow(subject string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.counts[subject]++
	return r.counts[subject] <= r.maxHits
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
	store       *InMemoryStore
	queue       *InMemoryQueue
	rateLimiter *RateLimiter
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
	return &Service{
		secret:      []byte(secret),
		nonceTTL:    10 * time.Minute,
		store:       NewInMemoryStore(),
		queue:       &InMemoryQueue{},
		rateLimiter: NewRateLimiter(5),
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

	if !s.store.AddNonce(msg.Nonce, s.nonceTTL) {
		s.audit.Append("BREVIO.security.webhook.replay_blocked.v1")
		http.Error(w, ErrReplayDetected.Error(), http.StatusConflict)
		return
	}

	if !s.rateLimiter.Allow(msg.UserChannelID) {
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

	w.WriteHeader(http.StatusAccepted)
	_, _ = w.Write([]byte(`{"status":"accepted"}`))
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

func (s *Service) HandleHealth(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ok"))
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

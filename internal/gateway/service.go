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

type IngressTurn struct {
	ID            uuid.UUID
	WorkspaceID   uuid.UUID
	UserChannelID string
	DedupHash     string
	Payload       []byte
	CreatedAt     time.Time
}

type QueueMessage struct {
	IngressTurnID uuid.UUID
	GroupKey      string
	DedupKey      string
	Payload       []byte
}

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
	for k, expiresAt := range s.byNonce {
		if now.After(expiresAt) {
			delete(s.byNonce, k)
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

type WorkspaceRouter struct {
	mu       sync.RWMutex
	bindings map[string]uuid.UUID
}

func NewWorkspaceRouter() *WorkspaceRouter {
	return &WorkspaceRouter{bindings: map[string]uuid.UUID{}}
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

type Service struct {
	secret      []byte
	nonceTTL    time.Duration
	store       *InMemoryStore
	queue       *InMemoryQueue
	rateLimiter *RateLimiter
	audit       *AuditLog
	router      *WorkspaceRouter
}

func NewService(secret string) *Service {
	return &Service{
		secret:      []byte(secret),
		nonceTTL:    10 * time.Minute,
		store:       NewInMemoryStore(),
		queue:       &InMemoryQueue{},
		rateLimiter: NewRateLimiter(5),
		audit:       &AuditLog{},
		router:      NewWorkspaceRouter(),
	}
}

type inboundMessage struct {
	Channel           string `json:"channel"`
	ChannelIdentifier string `json:"channel_identifier"`
	UserChannelID     string `json:"user_channel_id"`
	Nonce             string `json:"nonce"`
	Message           string `json:"message"`
	InteractiveReply  string `json:"interactive_reply"`
	DiscoveryAnswer   string `json:"discovery_answer"`
	AudioURL          string `json:"audio_url"`
}

func signatureFor(secret, payload []byte) string {
	mac := hmac.New(sha256.New, secret)
	mac.Write(payload)
	return hex.EncodeToString(mac.Sum(nil))
}

func (s *Service) validateSignature(payload []byte, signature string) error {
	expected := signatureFor(s.secret, payload)
	if !hmac.Equal([]byte(expected), []byte(strings.ToLower(signature))) {
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
	if err := s.validateSignature(body, signature); err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}

	var msg inboundMessage
	if err := json.Unmarshal(body, &msg); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if !s.store.AddNonce(msg.Nonce, s.nonceTTL) {
		s.audit.Append("BREVIO.security.webhook.replay_blocked.v1")
		http.Error(w, ErrReplayDetected.Error(), http.StatusConflict)
		return
	}

	if !s.rateLimiter.Allow(msg.UserChannelID) {
		http.Error(w, ErrRateLimited.Error(), http.StatusTooManyRequests)
		return
	}

	workspaceID, err := s.router.Resolve(msg.Channel, msg.ChannelIdentifier)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	turnID, err := uuid.NewV7()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	turn := IngressTurn{
		ID:            turnID,
		WorkspaceID:   workspaceID,
		UserChannelID: msg.UserChannelID,
		DedupHash:     dedupHash(body),
		Payload:       body,
		CreatedAt:     time.Now().UTC(),
	}
	if inserted := s.store.InsertIngressTurn(turn); !inserted {
		w.WriteHeader(http.StatusOK)
		return
	}

	s.queue.Enqueue(QueueMessage{
		IngressTurnID: turn.ID,
		GroupKey:      msg.UserChannelID,
		DedupKey:      turn.ID.String(),
		Payload:       body,
	})

	w.WriteHeader(http.StatusAccepted)
	_, _ = w.Write([]byte(`{"status":"accepted"}`))
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
	w.WriteHeader(http.StatusAccepted)
	_, _ = w.Write([]byte(`{"status":"queued"}`))
}

func (s *Service) ParseInteractiveReply(raw string) string {
	return strings.TrimSpace(raw)
}

func (s *Service) ParseDiscoveryAnswer(raw string) string {
	return strings.TrimSpace(raw)
}

func (s *Service) PreprocessVoice(_ context.Context, audioURL string) string {
	if audioURL == "" {
		return ""
	}
	return "transcript:" + audioURL
}

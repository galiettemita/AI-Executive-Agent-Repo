package gateway

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/brevio/brevio/internal/database"
	"github.com/brevio/brevio/internal/outbox"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ProdDeps holds all production dependencies for the gateway service.
type ProdDeps struct {
	DB             database.Querier
	Pool           *pgxpool.Pool
	WebhookSecret  string
	ServiceOptions ServiceOptions
	AttachUploader AttachmentUploader
	OutboxService  *outbox.Service
}

// ProdService wraps the core Service with production-grade persistence.
// It delegates HTTP handling to the embedded Service but replaces in-memory
// stores with pgx-backed repositories.
type ProdService struct {
	*Service

	ingressRepo     IngressTurnRepository
	dedupRepo       DeduplicationRepository
	queueRepo       MessageQueueRepository
	idempotencyRepo IdempotencyRepository
	outboxService   *outbox.Service
	pool            *pgxpool.Pool
}

// NewServiceProd creates a production gateway service backed by PostgreSQL.
// All in-memory stores are replaced with pgx repositories.
func NewServiceProd(deps ProdDeps) (*ProdService, error) {
	if deps.DB == nil {
		return nil, fmt.Errorf("gateway: DB querier is required for production")
	}
	if deps.Pool == nil {
		return nil, fmt.Errorf("gateway: pgx pool is required for production")
	}
	if deps.WebhookSecret == "" {
		return nil, fmt.Errorf("gateway: webhook secret is required for production")
	}

	base := NewServiceWithOptions(deps.WebhookSecret, deps.ServiceOptions)

	if deps.AttachUploader != nil {
		base.uploader = deps.AttachUploader
	}

	ps := &ProdService{
		Service:         base,
		ingressRepo:     NewPgIngressTurnRepository(deps.DB),
		dedupRepo:       NewPgDeduplicationRepository(deps.DB),
		queueRepo:       NewPgMessageQueueRepository(deps.DB),
		idempotencyRepo: NewPgIdempotencyRepository(deps.DB),
		outboxService:   deps.OutboxService,
		pool:            deps.Pool,
	}

	return ps, nil
}

// HandleInbound overrides the base Service's HandleInbound to persist turns,
// envelopes, and queue messages in PostgreSQL instead of in-memory stores.
func (ps *ProdService) HandleInbound(w http.ResponseWriter, r *http.Request) {
	body, err := readAndValidateInbound(ps.Service, w, r)
	if err != nil {
		return // error already written to response
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

	ctx := r.Context()

	// DB-backed idempotency check.
	idempotencyKey := gatewayIdempotencyKey(msg.Channel, msg.ChannelMessageID)
	if idempotencyKey != "" {
		if statusCode, cachedBody, ok := ps.idempotencyRepo.Get(ctx, idempotencyKey); ok {
			ps.audit.Append("BREVIO.ingress.idempotent_replay.v1")
			w.WriteHeader(statusCode)
			_, _ = w.Write(cachedBody)
			return
		}
	}

	// DB-backed nonce replay protection.
	nonceUsed, err := ps.dedupRepo.IsNonceUsed(ctx, "", msg.Nonce)
	if err == nil && nonceUsed {
		ps.audit.Append("BREVIO.security.webhook.replay_blocked.v1")
		http.Error(w, ErrReplayDetected.Error(), http.StatusConflict)
		return
	}

	if !ps.rateLimiter.Allow(msg.Channel+"::"+msg.UserChannelID, msg.UserTier) {
		ps.audit.Append("BREVIO.gateway.rate_limited.v1")
		http.Error(w, ErrRateLimited.Error(), http.StatusTooManyRequests)
		return
	}

	workspaceID, _, err := ps.router.ResolveForInbound(msg.Channel, msg.ChannelIdentifier)
	if err != nil {
		ps.audit.Append("BREVIO.security.identity.verification_failed.v1")
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	turnID, err := uuid.NewV7()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	now := ps.nowUTC()

	attachmentRefs := make([]AttachmentReference, 0, len(msg.Attachments))
	for _, attachment := range msg.Attachments {
		if err := ValidateAttachmentInput(attachment); err != nil {
			ps.audit.Append("BREVIO.ingress.attachment_rejected.v1")
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		ref, uploadErr := ps.uploader.Upload(ctx, workspaceID, attachment)
		if uploadErr != nil {
			http.Error(w, uploadErr.Error(), http.StatusBadGateway)
			return
		}
		attachmentRefs = append(attachmentRefs, ref)
	}

	transcript := ps.PreprocessVoice(ctx, msg.AudioURL)
	userID, err := ps.resolveOrCreateUserID(msg.Channel, msg.UserChannelID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	sessionID, err := ps.resolveSessionID(msg.Channel, msg.UserChannelID, now)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	envelope, err := BuildMessageEnvelope(BuildMessageEnvelopeInput{
		ID:               turnID,
		Channel:          msg.Channel,
		UserID:           userID,
		Timestamp:        now,
		MessageText:      msg.Message,
		Transcript:       transcript,
		AudioURL:         msg.AudioURL,
		VoiceDurationMS:  msg.VoiceDurationMS,
		Attachments:      attachmentRefs,
		ChannelMessageID: msg.ChannelMessageID,
		ReplyTo:          msg.ReplyTo,
		SessionID:        sessionID,
		UserProfileHash:  DeriveUserProfileHash(workspaceID, userID),
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	envelopePayload, err := json.Marshal(envelope)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	turn := IngressTurn{
		ID:                     turnID,
		WorkspaceID:            workspaceID,
		UserChannelID:          msg.UserChannelID,
		DedupHash:              dedupHash(body),
		Payload:                body,
		ParsedInteractiveReply: ps.ParseInteractiveReply(msg.InteractiveReply),
		ParsedDiscoveryAnswer:  ps.ParseDiscoveryAnswer(msg.DiscoveryAnswer),
		Transcript:             transcript,
		Attachments:            attachmentRefs,
		CreatedAt:              now,
	}

	identityEnv := ChannelIdentityEnvelope{
		IngressTurnID:      turn.ID,
		ChannelType:        msg.Channel,
		ClaimedIdentifier:  msg.ChannelIdentifier,
		VerificationMethod: "webhook_signature+sender_binding",
		VerificationResult: "verified",
		RawSignature:       r.Header.Get("X-Signature"),
		VerifiedAt:         now,
	}

	// Persist turn to DB (dedup via ON CONFLICT).
	inserted, err := ps.ingressRepo.InsertTurn(ctx, &turn)
	if err != nil {
		http.Error(w, fmt.Sprintf("persist ingress turn: %v", err), http.StatusInternalServerError)
		return
	}
	if !inserted {
		ps.audit.Append("BREVIO.ingress.duplicate_dropped.v1")
		if idempotencyKey != "" {
			_ = ps.idempotencyRepo.Set(ctx, idempotencyKey, http.StatusOK, []byte(`{"status":"duplicate_dropped"}`), ps.idempTTL)
		}
		w.WriteHeader(http.StatusOK)
		return
	}

	// Persist identity envelope.
	_ = ps.ingressRepo.InsertIdentityEnvelope(ctx, workspaceID, &identityEnv)

	// Store nonce for replay protection.
	_ = ps.dedupRepo.StoreNonce(ctx, workspaceID.String(), msg.Nonce, turnID.String())

	ps.audit.Append("BREVIO.ingress.received.v1")

	// Enqueue to durable DB queue.
	queueMsg := &QueueMessage{
		IngressTurnID:     turn.ID,
		WorkspaceID:       turn.WorkspaceID,
		Channel:           strings.ToLower(strings.TrimSpace(msg.Channel)),
		ChannelIdentifier: strings.TrimSpace(msg.ChannelIdentifier),
		UserChannelID:     msg.UserChannelID,
		GroupKey:           msg.UserChannelID,
		DedupKey:          turn.ID.String(),
		Payload:           envelopePayload,
	}
	if err := ps.queueRepo.Enqueue(ctx, queueMsg); err != nil {
		http.Error(w, fmt.Sprintf("enqueue message: %v", err), http.StatusInternalServerError)
		return
	}

	acceptedBody := []byte(`{"status":"accepted"}`)
	if idempotencyKey != "" {
		_ = ps.idempotencyRepo.Set(ctx, idempotencyKey, http.StatusAccepted, acceptedBody, ps.idempTTL)
	}
	w.WriteHeader(http.StatusAccepted)
	_, _ = w.Write(acceptedBody)
}

// HandleOutboundSend overrides the base to enqueue via the transactional outbox
// instead of the in-memory outbox slice.
func (ps *ProdService) HandleOutboundSend(w http.ResponseWriter, r *http.Request) {
	if ps.outboxService == nil {
		// Fall back to base in-memory outbox if no outbox service configured.
		ps.Service.HandleOutboundSend(w, r)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	var ob outboundSendRequest
	if err := json.Unmarshal(body, &ob); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if ob.WorkspaceID == "" || ob.Channel == "" || ob.ChannelIdentifier == "" || ob.Body == "" {
		http.Error(w, "workspace_id, channel, channel_identifier, and body are required", http.StatusBadRequest)
		return
	}
	if _, err := uuid.Parse(ob.WorkspaceID); err != nil {
		http.Error(w, "workspace_id must be a valid uuid", http.StatusBadRequest)
		return
	}

	outboundID, err := uuid.NewV7()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	payloadJSON, err := json.Marshal(map[string]string{
		"channel":            ob.Channel,
		"channel_identifier": ob.ChannelIdentifier,
		"body":               ob.Body,
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	ctx := r.Context()
	tx, err := ps.pool.Begin(ctx)
	if err != nil {
		http.Error(w, fmt.Sprintf("begin tx: %v", err), http.StatusInternalServerError)
		return
	}
	defer tx.Rollback(ctx)

	entry := outbox.OutboxEntry{
		ID:            outboundID.String(),
		WorkspaceID:   ob.WorkspaceID,
		AggregateType: "outbound_dispatch",
		AggregateID:   outboundID.String(),
		EventType:     "outbound_send",
		Payload:       payloadJSON,
		Target:        ob.Channel,
		Status:        outbox.StatusPending,
		MaxAttempts:   5,
		CreatedAt:     time.Now().UTC(),
	}
	if err := ps.outboxService.Enqueue(ctx, tx, entry); err != nil {
		http.Error(w, fmt.Sprintf("outbox enqueue: %v", err), http.StatusInternalServerError)
		return
	}

	if err := tx.Commit(ctx); err != nil {
		http.Error(w, fmt.Sprintf("commit: %v", err), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusAccepted)
	_, _ = w.Write([]byte(`{"status":"queued"}`))
}

// readAndValidateInbound reads the request body and validates signature/iMessage API key.
func readAndValidateInbound(s *Service, w http.ResponseWriter, r *http.Request) ([]byte, error) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return nil, err
	}
	if err := s.validateIMessageAPIKey(r); err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return nil, err
	}

	signature := r.Header.Get("X-Signature")
	if signature == "" {
		signature = r.Header.Get("X-Hub-Signature-256")
	}
	if err := s.validateSignature(body, signature); err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return nil, err
	}
	return body, nil
}

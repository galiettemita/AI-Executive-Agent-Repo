package gateway

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
)

func signedRequestBody(secret, path string, payload []byte) *http.Request {
	req := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(payload))
	req.Header.Set("X-Signature", signatureFor([]byte(secret), payload))
	return req
}

func TestInvalidSignatureReturns401AndAuditEntry(t *testing.T) {
	t.Parallel()

	svc := NewService("test-secret")
	svc.router.Bind("whatsapp", "+15550001111", uuid.MustParse("018f3f6a-9a0f-7cc6-8f2f-1f0f2d2f2d2f"))
	payload := []byte(`{"channel":"whatsapp","channel_identifier":"+15550001111","user_channel_id":"u1","nonce":"n1","message":"hello"}`)

	req := httptest.NewRequest(http.MethodPost, "/v1/gateway/webhook/whatsapp", bytes.NewReader(payload))
	req.Header.Set("X-Signature", "deadbeef")
	resp := httptest.NewRecorder()

	svc.HandleInbound(resp, req)
	if resp.Code != http.StatusUnauthorized {
		t.Fatalf("unexpected status: %d", resp.Code)
	}
	if svc.audit.Count() != 1 {
		t.Fatalf("expected 1 audit entry, got %d", svc.audit.Count())
	}
	if !containsString(svc.AuditEntries(), "BREVIO.security.webhook.signature_invalid.v1") {
		t.Fatalf("expected invalid signature event in audit log, got %v", svc.AuditEntries())
	}
}

func TestReplayNonceRejected(t *testing.T) {
	t.Parallel()

	svc := NewService("test-secret")
	svc.router.Bind("whatsapp", "+15550001111", uuid.MustParse("018f3f6a-9a0f-7cc6-8f2f-1f0f2d2f2d2f"))
	payload := []byte(`{"channel":"whatsapp","channel_identifier":"+15550001111","user_channel_id":"u1","nonce":"n1","message":"hello"}`)

	resp1 := httptest.NewRecorder()
	svc.HandleInbound(resp1, signedRequestBody("test-secret", "/v1/gateway/webhook/whatsapp", payload))
	if resp1.Code != http.StatusAccepted {
		t.Fatalf("unexpected initial status: %d", resp1.Code)
	}

	resp2 := httptest.NewRecorder()
	svc.HandleInbound(resp2, signedRequestBody("test-secret", "/v1/gateway/webhook/whatsapp", payload))
	if resp2.Code != http.StatusConflict {
		t.Fatalf("expected replay conflict, got %d", resp2.Code)
	}
	if !containsString(svc.AuditEntries(), "BREVIO.security.webhook.replay_blocked.v1") {
		t.Fatalf("expected replay-blocked event in audit log, got %v", svc.AuditEntries())
	}
}

func TestValidMessageCreatesIngressAndQueue(t *testing.T) {
	t.Parallel()

	svc := NewService("test-secret")
	svc.router.Bind("whatsapp", "+15550001111", uuid.MustParse("018f3f6a-9a0f-7cc6-8f2f-1f0f2d2f2d2f"))
	payload := []byte(`{"channel":"whatsapp","channel_identifier":"+15550001111","user_channel_id":"u1","nonce":"n2","message":"hello"}`)

	resp := httptest.NewRecorder()
	svc.HandleInbound(resp, signedRequestBody("test-secret", "/v1/gateway/webhook/whatsapp", payload))
	if resp.Code != http.StatusAccepted {
		t.Fatalf("unexpected status: %d", resp.Code)
	}
	if svc.store.TurnCount() != 1 {
		t.Fatalf("expected 1 ingress turn, got %d", svc.store.TurnCount())
	}
	if svc.queue.Count() != 1 {
		t.Fatalf("expected 1 queue message, got %d", svc.queue.Count())
	}
	if !containsString(svc.AuditEntries(), "BREVIO.ingress.received.v1") {
		t.Fatalf("expected ingress received event in audit log, got %v", svc.AuditEntries())
	}
	if svc.ChannelIdentityEnvelopeCount() != 1 {
		t.Fatalf("expected one identity envelope, got %d", svc.ChannelIdentityEnvelopeCount())
	}

	msg, ok := svc.PopQueueMessage()
	if !ok {
		t.Fatal("expected queue message")
	}
	if msg.GroupKey != "u1" {
		t.Fatalf("unexpected fifo group key: %s", msg.GroupKey)
	}
	if msg.DedupKey != msg.IngressTurnID.String() {
		t.Fatalf("unexpected fifo dedup key: got=%s want=%s", msg.DedupKey, msg.IngressTurnID.String())
	}
	envelope, err := DecodeMessageEnvelope(msg.Payload)
	if err != nil {
		t.Fatalf("decode message envelope: %v", err)
	}
	if envelope.Channel != "WHATSAPP" {
		t.Fatalf("unexpected envelope channel: %s", envelope.Channel)
	}
	if envelope.Content.Type != "TEXT" {
		t.Fatalf("unexpected envelope content type: %s", envelope.Content.Type)
	}
	if envelope.Metadata.ChannelMessageID == "" {
		t.Fatalf("expected channel_message_id in envelope: %+v", envelope.Metadata)
	}
	if envelope.Metadata.SessionID == "" {
		t.Fatalf("expected session_id in envelope: %+v", envelope.Metadata)
	}
}

func TestUnboundChannelRejectedWithIdentityFailureEvent(t *testing.T) {
	t.Parallel()

	svc := NewService("test-secret")
	payload := []byte(`{"channel":"whatsapp","channel_identifier":"+15559998888","user_channel_id":"u_missing","nonce":"n_missing","message":"hello"}`)

	resp := httptest.NewRecorder()
	svc.HandleInbound(resp, signedRequestBody("test-secret", "/v1/gateway/webhook/whatsapp", payload))
	if resp.Code != http.StatusBadRequest {
		t.Fatalf("expected bad request for unbound channel, got %d", resp.Code)
	}
	if !containsString(svc.AuditEntries(), "BREVIO.security.identity.verification_failed.v1") {
		t.Fatalf("expected identity verification failed event in audit log, got %v", svc.AuditEntries())
	}
}

func TestIMessageInboundUsesSamePipeline(t *testing.T) {
	t.Parallel()

	svc := NewService("test-secret")
	svc.router.Bind("imessage", "imsg:user-1", uuid.MustParse("018f3f6a-9a0f-7cc6-8f2f-1f0f2d2f2d2f"))
	payload := []byte(`{"channel":"imessage","channel_identifier":"imsg:user-1","user_channel_id":"imsg_u1","nonce":"n_imsg","message":"hello from iMessage"}`)

	resp := httptest.NewRecorder()
	req := signedRequestBody("test-secret", "/v1/gateway/webhook/imessage", payload)
	req.Header.Set("X-API-Key", "dev-imessage-key")
	svc.HandleInbound(resp, req)
	if resp.Code != http.StatusAccepted {
		t.Fatalf("expected imessage webhook to be accepted, got %d", resp.Code)
	}
	if svc.IngressTurnCount() != 1 {
		t.Fatalf("expected ingress turn count 1, got %d", svc.IngressTurnCount())
	}
}

func TestIMessageInboundMissingAPIKeyRejected(t *testing.T) {
	t.Parallel()

	svc := NewService("test-secret")
	svc.router.Bind("imessage", "imsg:user-2", uuid.MustParse("018f3f6a-9a0f-7cc6-8f2f-1f0f2d2f2d2f"))
	payload := []byte(`{"channel":"imessage","channel_identifier":"imsg:user-2","user_channel_id":"imsg_u2","nonce":"n_imsg_missing_key","message":"hello"}`)

	resp := httptest.NewRecorder()
	svc.HandleInbound(resp, signedRequestBody("test-secret", "/v1/gateway/webhook/imessage", payload))
	if resp.Code != http.StatusUnauthorized {
		t.Fatalf("expected unauthorized when imessage api key is missing, got %d", resp.Code)
	}
	if !containsString(svc.AuditEntries(), "BREVIO.security.imessage.api_key_invalid.v1") {
		t.Fatalf("expected imessage api-key audit event, got %v", svc.AuditEntries())
	}
}

func TestAttachmentInteractiveDiscoveryAndVoicePreprocess(t *testing.T) {
	t.Parallel()

	svc := NewService("test-secret")
	workspaceID := uuid.MustParse("018f3f6a-9a0f-7cc6-8f2f-1f0f2d2f2d2f")
	svc.router.Bind("whatsapp", "+15550001111", workspaceID)

	payload := []byte(`{
		"channel":"whatsapp",
		"channel_identifier":"+15550001111",
		"user_channel_id":"u1",
		"nonce":"n_attach",
		"message":"please review",
		"interactive_reply":"{\"button_reply\":{\"id\":\"approve\",\"title\":\"Approve\"}}",
		"discovery_answer":"{\"question\":\"team_size\",\"answer\":\"12\"}",
		"audio_url":"https://cdn.example.com/audio-note.ogg",
		"attachments":[{"url":"https://files.example.com/report.pdf","mime_type":"application/pdf","filename":"report.pdf","size_bytes":1024}]
	}`)

	resp := httptest.NewRecorder()
	svc.HandleInbound(resp, signedRequestBody("test-secret", "/v1/gateway/webhook/whatsapp", payload))
	if resp.Code != http.StatusAccepted {
		t.Fatalf("expected accepted response, got %d", resp.Code)
	}

	turn, ok := svc.LastIngressTurn()
	if !ok {
		t.Fatal("expected last ingress turn")
	}
	if turn.ParsedInteractiveReply != "APPROVE" {
		t.Fatalf("unexpected interactive parse output: %s", turn.ParsedInteractiveReply)
	}
	if turn.ParsedDiscoveryAnswer != "question=team_size;answer=12" {
		t.Fatalf("unexpected discovery parse output: %s", turn.ParsedDiscoveryAnswer)
	}
	if turn.Transcript != "transcript:https://cdn.example.com/audio-note.ogg" {
		t.Fatalf("unexpected transcript: %s", turn.Transcript)
	}
	if len(turn.Attachments) != 1 {
		t.Fatalf("expected one attachment reference, got %d", len(turn.Attachments))
	}
	if turn.Attachments[0].S3URI == "" {
		t.Fatal("expected attachment to be uploaded and linked with s3 uri")
	}
	msg, ok := svc.PopQueueMessage()
	if !ok {
		t.Fatal("expected queued envelope for voice turn")
	}
	envelope, err := DecodeMessageEnvelope(msg.Payload)
	if err != nil {
		t.Fatalf("decode queued envelope: %v", err)
	}
	if envelope.Content.Type != "VOICE" {
		t.Fatalf("expected VOICE content type, got %s", envelope.Content.Type)
	}
	if envelope.Content.Text != "transcript:https://cdn.example.com/audio-note.ogg" {
		t.Fatalf("unexpected envelope transcription text: %s", envelope.Content.Text)
	}
	if envelope.Content.MediaURL != "https://cdn.example.com/audio-note.ogg" {
		t.Fatalf("unexpected envelope media_url: %s", envelope.Content.MediaURL)
	}
}

func TestAttachmentRejectedWhenInvalidMimeOrSize(t *testing.T) {
	t.Parallel()

	svc := NewService("test-secret")
	workspaceID := uuid.MustParse("018f3f6a-9a0f-7cc6-8f2f-1f0f2d2f2d2f")
	svc.router.Bind("whatsapp", "+15550001111", workspaceID)

	payload := []byte(`{
		"channel":"whatsapp",
		"channel_identifier":"+15550001111",
		"user_channel_id":"u1",
		"nonce":"n_invalid_attach",
		"message":"please review",
		"attachments":[{"url":"https://files.example.com/malware.exe","mime_type":"application/x-msdownload","filename":"malware.exe","size_bytes":1024}]
	}`)

	resp := httptest.NewRecorder()
	svc.HandleInbound(resp, signedRequestBody("test-secret", "/v1/gateway/webhook/whatsapp", payload))
	if resp.Code != http.StatusBadRequest {
		t.Fatalf("expected bad request for invalid attachment, got %d", resp.Code)
	}
	if !containsString(svc.AuditEntries(), "BREVIO.ingress.attachment_rejected.v1") {
		t.Fatalf("expected attachment_rejected audit event, got %v", svc.AuditEntries())
	}
}

func TestInteractiveReplyParserTextIntents(t *testing.T) {
	t.Parallel()

	svc := NewService("test-secret")
	cases := []struct {
		raw  string
		want string
	}{
		{raw: "yes", want: "APPROVE"},
		{raw: "No", want: "DENY"},
		{raw: "undo", want: "UNDO"},
		{raw: "edit move it to 4pm", want: "EDIT:move it to 4pm"},
		{raw: "2", want: "OPTION_INDEX:2"},
	}
	for _, tc := range cases {
		got := svc.ParseInteractiveReply(tc.raw)
		if got != tc.want {
			t.Fatalf("unexpected parse for %q: got=%q want=%q", tc.raw, got, tc.want)
		}
	}
}

func TestRateLimitedUserGets429(t *testing.T) {
	t.Parallel()

	svc := NewService("test-secret")
	svc.router.Bind("whatsapp", "+15550001111", uuid.MustParse("018f3f6a-9a0f-7cc6-8f2f-1f0f2d2f2d2f"))

	for i := 0; i < 30; i++ {
		payload := []byte(fmt.Sprintf(`{"channel":"whatsapp","channel_identifier":"+15550001111","user_channel_id":"u1","user_tier":"free","nonce":"n_%d","message":"hello"}`, i))
		resp := httptest.NewRecorder()
		svc.HandleInbound(resp, signedRequestBody("test-secret", "/v1/gateway/webhook/whatsapp", payload))
		if resp.Code != http.StatusAccepted {
			t.Fatalf("unexpected status at iteration %d: %d", i, resp.Code)
		}
	}

	payload := []byte(`{"channel":"whatsapp","channel_identifier":"+15550001111","user_channel_id":"u1","user_tier":"free","nonce":"n_limit","message":"hello"}`)
	resp := httptest.NewRecorder()
	svc.HandleInbound(resp, signedRequestBody("test-secret", "/v1/gateway/webhook/whatsapp", payload))
	if resp.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429, got %d", resp.Code)
	}
}

func TestEnterpriseTierBypassesRateLimit(t *testing.T) {
	t.Parallel()

	svc := NewService("test-secret")
	svc.router.Bind("whatsapp", "+15550001111", uuid.MustParse("018f3f6a-9a0f-7cc6-8f2f-1f0f2d2f2d2f"))

	for i := 0; i < 150; i++ {
		payload := []byte(fmt.Sprintf(`{"channel":"whatsapp","channel_identifier":"+15550001111","user_channel_id":"u-enterprise","user_tier":"enterprise","nonce":"enterprise_%d","message":"hello"}`, i))
		resp := httptest.NewRecorder()
		svc.HandleInbound(resp, signedRequestBody("test-secret", "/v1/gateway/webhook/whatsapp", payload))
		if resp.Code != http.StatusAccepted {
			t.Fatalf("unexpected status at iteration %d: %d", i, resp.Code)
		}
	}
}

func TestChannelMessageIdIdempotencyReplayReturnsCachedResponse(t *testing.T) {
	t.Parallel()

	svc := NewService("test-secret")
	svc.router.Bind("whatsapp", "+15550001111", uuid.MustParse("018f3f6a-9a0f-7cc6-8f2f-1f0f2d2f2d2f"))

	firstPayload := []byte(`{"channel":"whatsapp","channel_identifier":"+15550001111","user_channel_id":"u1","channel_message_id":"wamid.001","nonce":"n_idem_1","message":"hello first"}`)
	firstResp := httptest.NewRecorder()
	svc.HandleInbound(firstResp, signedRequestBody("test-secret", "/v1/gateway/webhook/whatsapp", firstPayload))
	if firstResp.Code != http.StatusAccepted {
		t.Fatalf("unexpected first status: %d", firstResp.Code)
	}
	if firstResp.Body.String() != `{"status":"accepted"}` {
		t.Fatalf("unexpected first response body: %s", firstResp.Body.String())
	}
	if svc.IngressTurnCount() != 1 || svc.QueueMessageCount() != 1 {
		t.Fatalf("expected single turn+queue after first message, turns=%d queue=%d", svc.IngressTurnCount(), svc.QueueMessageCount())
	}

	secondPayload := []byte(`{"channel":"whatsapp","channel_identifier":"+15550001111","user_channel_id":"u1","channel_message_id":"wamid.001","nonce":"n_idem_2","message":"hello replay"}`)
	secondResp := httptest.NewRecorder()
	svc.HandleInbound(secondResp, signedRequestBody("test-secret", "/v1/gateway/webhook/whatsapp", secondPayload))
	if secondResp.Code != http.StatusAccepted {
		t.Fatalf("unexpected replay status: %d", secondResp.Code)
	}
	if secondResp.Body.String() != `{"status":"accepted"}` {
		t.Fatalf("unexpected replay response body: %s", secondResp.Body.String())
	}
	if svc.IngressTurnCount() != 1 || svc.QueueMessageCount() != 1 {
		t.Fatalf("expected idempotent replay to skip write path, turns=%d queue=%d", svc.IngressTurnCount(), svc.QueueMessageCount())
	}
	if !containsString(svc.AuditEntries(), "BREVIO.ingress.idempotent_replay.v1") {
		t.Fatalf("expected idempotent replay audit event, got %v", svc.AuditEntries())
	}
}

func TestInjectToolCallAccepted(t *testing.T) {
	t.Parallel()

	svc := NewService("test-secret")
	mux := NewMux(svc)

	payload := []byte(`{"workspace_id":"018f3f6a-9a0f-7cc6-8f2f-1f0f2d2f2d2f","ingress_turn_id":"018f3f6a-9a0f-7cc6-8f2f-1f0f2d2f2d2f","tool_key":"calendar.create_event","arguments":{"title":"Standup"}}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/gateway/inject/tool_call", bytes.NewReader(payload))
	resp := httptest.NewRecorder()

	mux.ServeHTTP(resp, req)
	if resp.Code != http.StatusAccepted {
		t.Fatalf("unexpected status: %d", resp.Code)
	}
	if svc.InjectedToolCallCount() != 1 {
		t.Fatalf("expected one injected tool call, got %d", svc.InjectedToolCallCount())
	}
}

func TestDuplicateIngressDropsWithCanonicalEvent(t *testing.T) {
	t.Parallel()

	svc := NewService("test-secret")
	workspaceID := uuid.MustParse("018f3f6a-9a0f-7cc6-8f2f-1f0f2d2f2d2f")
	svc.router.Bind("whatsapp", "+15550001111", workspaceID)

	payload := []byte(`{"channel":"whatsapp","channel_identifier":"+15550001111","user_channel_id":"u1","nonce":"n_dedup","message":"hello"}`)
	svc.store.InsertIngressTurn(IngressTurn{
		ID:            uuid.MustParse("018f3f6a-9a0f-7cc6-8f2f-1f0f2d2f2d2e"),
		WorkspaceID:   workspaceID,
		UserChannelID: "u1",
		DedupHash:     dedupHash(payload),
		Payload:       payload,
		CreatedAt:     time.Now().UTC(),
	})

	resp := httptest.NewRecorder()
	svc.HandleInbound(resp, signedRequestBody("test-secret", "/v1/gateway/webhook/whatsapp", payload))
	if resp.Code != http.StatusOK {
		t.Fatalf("expected dedup drop status 200, got %d", resp.Code)
	}
	if !containsString(svc.AuditEntries(), "BREVIO.ingress.duplicate_dropped.v1") {
		t.Fatalf("expected duplicate-dropped event in audit log, got %v", svc.AuditEntries())
	}
}

func TestWhatsAppVerificationHandshake(t *testing.T) {
	t.Parallel()

	svc := NewService("test-secret")
	req := httptest.NewRequest(http.MethodGet, "/v1/gateway/webhook/whatsapp?hub.challenge=abc123", nil)
	resp := httptest.NewRecorder()

	svc.HandleWhatsAppVerification(resp, req)
	if resp.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d", resp.Code)
	}
	if body := resp.Body.String(); body != "abc123" {
		t.Fatalf("unexpected challenge body: %s", body)
	}
}

func TestOutboundSendQueuesRequest(t *testing.T) {
	t.Parallel()

	svc := NewService("test-secret")
	mux := NewMux(svc)
	payload := []byte(`{"workspace_id":"018f3f6a-9a0f-7cc6-8f2f-1f0f2d2f2d2f","channel":"whatsapp","channel_identifier":"+15550001111","body":"outbound hello"}`)

	req := httptest.NewRequest(http.MethodPost, "/v1/gateway/outbound/send", bytes.NewReader(payload))
	resp := httptest.NewRecorder()
	mux.ServeHTTP(resp, req)
	if resp.Code != http.StatusAccepted {
		t.Fatalf("unexpected status: %d", resp.Code)
	}
	if svc.OutboundDispatchCount() != 1 {
		t.Fatalf("expected 1 outbound dispatch, got %d", svc.OutboundDispatchCount())
	}
}

func TestHealthEndpoints(t *testing.T) {
	t.Parallel()

	svc := NewService("test-secret")
	mux := NewMux(svc)

	for _, path := range []string{"/healthz/ready", "/healthz/live", "/health", "/health/deep"} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		resp := httptest.NewRecorder()
		mux.ServeHTTP(resp, req)
		if resp.Code != http.StatusOK {
			t.Fatalf("unexpected status for %s: %d", path, resp.Code)
		}
		if path == "/health" || path == "/health/deep" {
			var payload map[string]any
			if err := json.Unmarshal(resp.Body.Bytes(), &payload); err != nil {
				t.Fatalf("expected json response for %s: %v", path, err)
			}
			if payload["status"] != "healthy" {
				t.Fatalf("unexpected health payload for %s: %+v", path, payload)
			}
		}
	}
}

func TestWorkspaceRouterResolveForInboundFallbackAndAutobind(t *testing.T) {
	t.Parallel()

	router := NewWorkspaceRouter()
	workspaceID := uuid.MustParse("018f3f6a-9a0f-7cc6-8f2f-1f0f2d2f2d2f")
	router.SetUserDefaultWorkspace("whatsapp", "+15550002222", workspaceID)

	resolved, autoBound, err := router.ResolveForInbound("whatsapp", "+15550002222")
	if err != nil {
		t.Fatalf("resolve for inbound fallback: %v", err)
	}
	if resolved != workspaceID || !autoBound {
		t.Fatalf("unexpected fallback resolution: workspace=%s autoBound=%v", resolved, autoBound)
	}

	resolved, autoBound, err = router.ResolveForInbound("whatsapp", "+15550002222")
	if err != nil {
		t.Fatalf("resolve for inbound bound: %v", err)
	}
	if resolved != workspaceID || autoBound {
		t.Fatalf("unexpected bound resolution: workspace=%s autoBound=%v", resolved, autoBound)
	}
}

func TestSessionIDRotatesAfterFourHoursInactivity(t *testing.T) {
	t.Parallel()

	svc := NewService("test-secret")
	workspaceID := uuid.MustParse("018f3f6a-9a0f-7cc6-8f2f-1f0f2d2f2d2f")
	svc.router.Bind("whatsapp", "+15550003333", workspaceID)

	base := time.Date(2026, time.March, 3, 10, 0, 0, 0, time.UTC)
	now := base
	svc.SetNowForTest(func() time.Time { return now })

	makePayload := func(nonce string) []byte {
		return []byte(fmt.Sprintf(`{"channel":"whatsapp","channel_identifier":"+15550003333","user_channel_id":"u-session","nonce":"%s","message":"hello"}`, nonce))
	}

	resp1 := httptest.NewRecorder()
	svc.HandleInbound(resp1, signedRequestBody("test-secret", "/v1/gateway/webhook/whatsapp", makePayload("session_1")))
	if resp1.Code != http.StatusAccepted {
		t.Fatalf("unexpected first status: %d", resp1.Code)
	}
	msg1, ok := svc.PopQueueMessage()
	if !ok {
		t.Fatal("expected first queue message")
	}
	envelope1, err := DecodeMessageEnvelope(msg1.Payload)
	if err != nil {
		t.Fatalf("decode first envelope: %v", err)
	}

	now = base.Add(2 * time.Hour)
	resp2 := httptest.NewRecorder()
	svc.HandleInbound(resp2, signedRequestBody("test-secret", "/v1/gateway/webhook/whatsapp", makePayload("session_2")))
	if resp2.Code != http.StatusAccepted {
		t.Fatalf("unexpected second status: %d", resp2.Code)
	}
	msg2, ok := svc.PopQueueMessage()
	if !ok {
		t.Fatal("expected second queue message")
	}
	envelope2, err := DecodeMessageEnvelope(msg2.Payload)
	if err != nil {
		t.Fatalf("decode second envelope: %v", err)
	}
	if envelope2.Metadata.SessionID != envelope1.Metadata.SessionID {
		t.Fatalf("expected same session_id within 4h window: first=%s second=%s", envelope1.Metadata.SessionID, envelope2.Metadata.SessionID)
	}

	now = base.Add(7 * time.Hour)
	resp3 := httptest.NewRecorder()
	svc.HandleInbound(resp3, signedRequestBody("test-secret", "/v1/gateway/webhook/whatsapp", makePayload("session_3")))
	if resp3.Code != http.StatusAccepted {
		t.Fatalf("unexpected third status: %d", resp3.Code)
	}
	msg3, ok := svc.PopQueueMessage()
	if !ok {
		t.Fatal("expected third queue message")
	}
	envelope3, err := DecodeMessageEnvelope(msg3.Payload)
	if err != nil {
		t.Fatalf("decode third envelope: %v", err)
	}
	if envelope3.Metadata.SessionID == envelope1.Metadata.SessionID {
		t.Fatalf("expected rotated session_id after inactivity: first=%s third=%s", envelope1.Metadata.SessionID, envelope3.Metadata.SessionID)
	}
}

func containsString(items []string, needle string) bool {
	for _, item := range items {
		if item == needle {
			return true
		}
	}
	return false
}

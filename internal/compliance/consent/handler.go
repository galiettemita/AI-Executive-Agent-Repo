package consent

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/google/uuid"
	temporalclient "go.temporal.io/sdk/client"
)

// ConsentHandler provides HTTP handlers for the consent API endpoints.
type ConsentHandler struct {
	registry       *ConsentRegistry
	temporalClient temporalclient.Client
	taskQueue      string
	logger         *slog.Logger
}

// NewConsentHandler creates a new handler with all dependencies.
func NewConsentHandler(
	registry *ConsentRegistry,
	tc temporalclient.Client,
	taskQueue string,
	logger *slog.Logger,
) *ConsentHandler {
	return &ConsentHandler{
		registry:       registry,
		temporalClient: tc,
		taskQueue:      taskQueue,
		logger:         logger,
	}
}

// RegisterRoutes binds consent endpoints to the given mux.
func (h *ConsentHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /v1/consent/grant", h.HandleGrant)
	mux.HandleFunc("POST /v1/consent/revoke", h.HandleRevoke)
	mux.HandleFunc("GET /v1/consent/status", h.HandleStatus)
}

// HandleGrant handles POST /v1/consent/grant.
func (h *ConsentHandler) HandleGrant(w http.ResponseWriter, r *http.Request) {
	wsID, userID, err := extractAuthIDs(r)
	if err != nil {
		writeConsentJSON(w, http.StatusUnauthorized, map[string]string{"error": "authentication required"})
		return
	}

	var req struct {
		Purpose     string `json:"purpose"`
		LawfulBasis string `json:"lawful_basis"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeConsentJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	if !ValidPurposes[ConsentPurpose(req.Purpose)] {
		writeConsentJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid purpose"})
		return
	}
	if !ValidBases[LawfulBasis(req.LawfulBasis)] {
		writeConsentJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid lawful_basis"})
		return
	}

	record, err := h.registry.GrantConsent(r.Context(), GrantConsentRequest{
		WorkspaceID: wsID,
		UserID:      userID,
		Purpose:     ConsentPurpose(req.Purpose),
		LawfulBasis: LawfulBasis(req.LawfulBasis),
	})
	if err != nil {
		writeConsentJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeConsentJSON(w, http.StatusCreated, record)
}

// HandleRevoke handles POST /v1/consent/revoke.
func (h *ConsentHandler) HandleRevoke(w http.ResponseWriter, r *http.Request) {
	wsID, userID, err := extractAuthIDs(r)
	if err != nil {
		writeConsentJSON(w, http.StatusUnauthorized, map[string]string{"error": "authentication required"})
		return
	}

	var req struct {
		Purpose string `json:"purpose"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeConsentJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	if !ValidPurposes[ConsentPurpose(req.Purpose)] {
		writeConsentJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid purpose"})
		return
	}

	if err := h.registry.RevokeConsent(r.Context(), wsID, userID, ConsentPurpose(req.Purpose)); err != nil {
		if err == ErrConsentNotFound {
			writeConsentJSON(w, http.StatusNotFound, map[string]string{"error": "no active consent found"})
			return
		}
		writeConsentJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	// Trigger revocation workflow.
	var workflowID string
	if h.temporalClient != nil {
		wfID, wfErr := StartRevocationWorkflow(h.temporalClient, h.taskQueue, RevocationInput{
			WorkspaceID: wsID,
			UserID:      userID,
			Purpose:     ConsentPurpose(req.Purpose),
			RevokedAt:   time.Now(),
		})
		if wfErr != nil {
			h.logger.Error("revocation_workflow_start_error", "error", wfErr)
		} else {
			workflowID = wfID
		}
	}

	writeConsentJSON(w, http.StatusOK, map[string]any{
		"revoked":              true,
		"erasure_workflow_id":  workflowID,
	})
}

// HandleStatus handles GET /v1/consent/status.
func (h *ConsentHandler) HandleStatus(w http.ResponseWriter, r *http.Request) {
	wsID, userID, err := extractAuthIDs(r)
	if err != nil {
		writeConsentJSON(w, http.StatusUnauthorized, map[string]string{"error": "authentication required"})
		return
	}

	consents, err := h.registry.GetActiveConsents(r.Context(), wsID, userID)
	if err != nil {
		writeConsentJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	type consentStatus struct {
		Purpose   string     `json:"purpose"`
		GrantedAt time.Time  `json:"granted_at"`
		ExpiresAt *time.Time `json:"expires_at,omitempty"`
	}

	statuses := make([]consentStatus, 0, len(consents))
	for _, c := range consents {
		statuses = append(statuses, consentStatus{
			Purpose:   string(c.Purpose),
			GrantedAt: c.GrantedAt,
			ExpiresAt: c.ExpiresAt,
		})
	}

	writeConsentJSON(w, http.StatusOK, map[string]any{
		"consents": statuses,
	})
}

// extractAuthIDs extracts workspace_id and user_id from request headers.
// In production this would use JWT claims; here we use headers for compatibility.
func extractAuthIDs(r *http.Request) (uuid.UUID, uuid.UUID, error) {
	wsIDStr := r.Header.Get("X-Workspace-ID")
	userIDStr := r.Header.Get("X-User-ID")

	if wsIDStr == "" || userIDStr == "" {
		return uuid.Nil, uuid.Nil, ErrConsentNotFound
	}

	wsID, err := uuid.Parse(wsIDStr)
	if err != nil {
		return uuid.Nil, uuid.Nil, err
	}

	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		return uuid.Nil, uuid.Nil, err
	}

	return wsID, userID, nil
}

func writeConsentJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

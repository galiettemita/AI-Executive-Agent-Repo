package self_modification_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	selfmod "github.com/brevio/brevio/internal/self_modification"
)

func setupTestMux() *http.ServeMux {
	svc := selfmod.NewService()
	mux := http.NewServeMux()
	selfmod.RegisterRoutes(mux, svc)
	return mux
}

func TestGetPolicy_NotFound_Returns404(t *testing.T) {
	t.Parallel()
	mux := setupTestMux()
	req := httptest.NewRequest("GET", "/v1/self-modification/policy/ws-new", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusNotFound, rr.Code)
}

func TestUpsertPolicy_Valid_Returns200(t *testing.T) {
	t.Parallel()
	mux := setupTestMux()
	policy := selfmod.Policy{MaxAllowedRisk: "elevated", Enabled: true}
	body, _ := json.Marshal(policy)
	req := httptest.NewRequest("PUT", "/v1/self-modification/policy/ws-test", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestEvaluateAction_Returns200(t *testing.T) {
	t.Parallel()
	mux := setupTestMux()
	actionReq := selfmod.ActionRequest{ActionKey: "calendar.create_event", RequestedRisk: "low"}
	body, _ := json.Marshal(actionReq)
	req := httptest.NewRequest("POST", "/v1/self-modification/evaluate/ws-test", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	require.Equal(t, http.StatusOK, rr.Code)
}

func TestListDecisions_Empty_Returns200(t *testing.T) {
	t.Parallel()
	mux := setupTestMux()
	req := httptest.NewRequest("GET", "/v1/self-modification/decisions/ws-test", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code)
}

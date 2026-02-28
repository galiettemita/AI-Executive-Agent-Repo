package rbac

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
)

func TestOperatorCannotPerformAdminAction(t *testing.T) {
	t.Parallel()

	svc := NewService()
	workspaceID := uuid.New()
	operatorID := uuid.New()
	if err := svc.BindRole(workspaceID, operatorID, RoleOperator); err != nil {
		t.Fatalf("bind role: %v", err)
	}
	if err := svc.CheckRoleAtLeast(workspaceID, operatorID, RoleAdmin); err == nil {
		t.Fatal("expected operator to be denied admin action")
	}
}

func TestOwnerCanPerformAllActions(t *testing.T) {
	t.Parallel()

	svc := NewService()
	workspaceID := uuid.New()
	ownerID := uuid.New()
	if err := svc.BindRole(workspaceID, ownerID, RoleOwner); err != nil {
		t.Fatalf("bind role: %v", err)
	}
	for _, required := range []string{RoleOperator, RoleAuditor, RoleDelegate, RoleAdmin, RoleOwner} {
		if err := svc.CheckRoleAtLeast(workspaceID, ownerID, required); err != nil {
			t.Fatalf("owner should satisfy %s: %v", required, err)
		}
	}
}

func TestRevokedRoleCannotPassChecks(t *testing.T) {
	t.Parallel()

	svc := NewService()
	workspaceID := uuid.New()
	userID := uuid.New()
	if err := svc.BindRole(workspaceID, userID, RoleAdmin); err != nil {
		t.Fatalf("bind role: %v", err)
	}
	svc.RevokeRole(workspaceID, userID)
	if err := svc.CheckRoleAtLeast(workspaceID, userID, RoleOperator); err == nil {
		t.Fatal("expected revoked role binding to fail checks")
	}
}

func TestRequireRoleMiddleware(t *testing.T) {
	t.Parallel()

	svc := NewService()
	workspaceID := uuid.New()
	adminID := uuid.New()
	operatorID := uuid.New()

	if err := svc.BindRole(workspaceID, adminID, RoleAdmin); err != nil {
		t.Fatalf("bind admin role: %v", err)
	}
	if err := svc.BindRole(workspaceID, operatorID, RoleOperator); err != nil {
		t.Fatalf("bind operator role: %v", err)
	}

	protected := svc.RequireRole(RoleAdmin, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))

	adminReq := httptest.NewRequest(http.MethodPost, "/v1/admin/action", nil)
	adminReq.Header.Set(HeaderWorkspaceID, workspaceID.String())
	adminReq.Header.Set(HeaderUserID, adminID.String())
	adminRec := httptest.NewRecorder()
	protected.ServeHTTP(adminRec, adminReq)
	if adminRec.Code != http.StatusNoContent {
		t.Fatalf("expected admin to pass middleware, got %d", adminRec.Code)
	}

	operatorReq := httptest.NewRequest(http.MethodPost, "/v1/admin/action", nil)
	operatorReq.Header.Set(HeaderWorkspaceID, workspaceID.String())
	operatorReq.Header.Set(HeaderUserID, operatorID.String())
	operatorRec := httptest.NewRecorder()
	protected.ServeHTTP(operatorRec, operatorReq)
	if operatorRec.Code != http.StatusForbidden {
		t.Fatalf("expected operator to be forbidden, got %d", operatorRec.Code)
	}
}

func TestRequireRoleMiddlewareRejectsInvalidHeaders(t *testing.T) {
	t.Parallel()

	svc := NewService()
	protected := svc.RequireRole(RoleAdmin, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))

	req := httptest.NewRequest(http.MethodPost, "/v1/admin/action", nil)
	req.Header.Set(HeaderWorkspaceID, "not-a-uuid")
	req.Header.Set(HeaderUserID, "also-not-a-uuid")
	rec := httptest.NewRecorder()
	protected.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected bad request for invalid headers, got %d", rec.Code)
	}
}

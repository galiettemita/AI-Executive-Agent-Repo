package rbac

import (
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

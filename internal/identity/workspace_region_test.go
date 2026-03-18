package identity

import (
	"testing"

	"github.com/google/uuid"
)

func TestEUWorkspaceRoutedToEUWest1(t *testing.T) {
	svc := NewService()

	// Create account and workspace.
	acct, err := svc.CreateAccount("pro", "active", "")
	if err != nil {
		t.Fatalf("CreateAccount: %v", err)
	}

	user, err := svc.CreateUser(acct.ID, "test@example.com", "", "A2", "UTC")
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}

	ws, err := svc.CreateWorkspace(acct.ID, user.ID, "test-ns", "", nil)
	if err != nil {
		t.Fatalf("CreateWorkspace: %v", err)
	}

	// Set region to eu-west-1.
	if err := svc.SetWorkspaceRegion(ws.ID, RegionEUWest1); err != nil {
		t.Fatalf("SetWorkspaceRegion: %v", err)
	}

	region, err := svc.GetWorkspaceRegion(ws.ID)
	if err != nil {
		t.Fatalf("GetWorkspaceRegion: %v", err)
	}
	if region != RegionEUWest1 {
		t.Errorf("Expected region=%s, got %s", RegionEUWest1, region)
	}

	if !svc.IsEUWorkspace(ws.ID) {
		t.Error("Expected IsEUWorkspace=true for eu-west-1 workspace")
	}
}

func TestDefaultWorkspaceRegion(t *testing.T) {
	svc := NewService()

	acct, _ := svc.CreateAccount("pro", "active", "")
	user, _ := svc.CreateUser(acct.ID, "test@example.com", "", "A2", "UTC")
	ws, _ := svc.CreateWorkspace(acct.ID, user.ID, "test-ns", "", nil)

	// Without setting region, should default to us-east-1.
	region, err := svc.GetWorkspaceRegion(ws.ID)
	if err != nil {
		t.Fatalf("GetWorkspaceRegion: %v", err)
	}
	if region != RegionUSEast1 {
		t.Errorf("Expected default region=%s, got %s", RegionUSEast1, region)
	}

	if svc.IsEUWorkspace(ws.ID) {
		t.Error("Expected IsEUWorkspace=false for default region workspace")
	}
}

func TestInvalidRegion(t *testing.T) {
	svc := NewService()

	acct, _ := svc.CreateAccount("pro", "active", "")
	user, _ := svc.CreateUser(acct.ID, "test@example.com", "", "A2", "UTC")
	ws, _ := svc.CreateWorkspace(acct.ID, user.ID, "test-ns", "", nil)

	err := svc.SetWorkspaceRegion(ws.ID, "ap-southeast-1")
	if err == nil {
		t.Error("Expected error for invalid region")
	}
}

func TestWorkspaceNotFound(t *testing.T) {
	svc := NewService()

	_, err := svc.GetWorkspaceRegion(uuid.New())
	if err == nil {
		t.Error("Expected error for non-existent workspace")
	}
}

func TestValidRegions(t *testing.T) {
	if !ValidRegions[RegionUSEast1] {
		t.Error("us-east-1 should be valid")
	}
	if !ValidRegions[RegionEUWest1] {
		t.Error("eu-west-1 should be valid")
	}
	if ValidRegions["ap-southeast-1"] {
		t.Error("ap-southeast-1 should not be valid")
	}
}

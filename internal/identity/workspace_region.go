package identity

import (
	"fmt"

	"github.com/google/uuid"
)

const (
	RegionUSEast1 = "us-east-1"
	RegionEUWest1 = "eu-west-1"
)

// ValidRegions is the set of allowed workspace regions.
var ValidRegions = map[string]bool{
	RegionUSEast1: true,
	RegionEUWest1: true,
}

// GetWorkspaceRegion returns the target region for a workspace.
// Returns "us-east-1" as default if no region is set.
func (s *Service) GetWorkspaceRegion(workspaceID uuid.UUID) (string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	ws, ok := s.workspaces[workspaceID]
	if !ok {
		return "", fmt.Errorf("workspace not found: %s", workspaceID)
	}

	if ws.Region == "" {
		return RegionUSEast1, nil
	}
	return ws.Region, nil
}

// SetWorkspaceRegion updates the region for a workspace.
func (s *Service) SetWorkspaceRegion(workspaceID uuid.UUID, region string) error {
	if !ValidRegions[region] {
		return fmt.Errorf("invalid region: %s (must be us-east-1 or eu-west-1)", region)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	ws, ok := s.workspaces[workspaceID]
	if !ok {
		return fmt.Errorf("workspace not found: %s", workspaceID)
	}

	ws.Region = region
	s.workspaces[workspaceID] = ws
	return nil
}

// IsEUWorkspace returns true if the workspace is routed to eu-west-1.
func (s *Service) IsEUWorkspace(workspaceID uuid.UUID) bool {
	region, err := s.GetWorkspaceRegion(workspaceID)
	if err != nil {
		return false
	}
	return region == RegionEUWest1
}
